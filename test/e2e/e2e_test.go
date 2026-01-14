/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/shaygef123/SleePod-controller/test/utils"
)

// namespace where the project is deployed in
const namespace = "sleepod-controller-system"

// serviceAccountName created for the project
const serviceAccountName = "sleepod-controller-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "sleepod-controller-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "sleepod-controller-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("cleaning up SleepPolicy resources")
		cmd = exec.Command("kubectl", "delete", "sleeppolicy", "--all", "-A")
		_, _ = utils.Run(cmd)

		By("waiting for all SleepPolicy resources to be deleted")
		Eventually(func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "sleeppolicy", "-A")
			output, err := utils.Run(cmd)
			if err != nil {
				return
			}
			if output == "" || output == "No resources found." {
				return
			}
			// If still exists after timeout, force remove finalizers
			if CurrentSpecReport().Failed() {
				By("Forcing finalizer removal on SleepPolicies")
				// List names and patch
				// Simplified: assume we might need to do this manually if really stuck
				// But here we rely on the Eventually timeout.
				// Actually, we should try to patch inside the cleanup if it takes too long?
				// For now, let's keep the wait, but if it fails, we proceed to nuke namespace.
			}
			g.Expect(output).To(ContainSubstring("No resources found"), "SleepPolicies still exist")
		}, 2*time.Minute, time.Second).Should(Succeed())

		By("cleaning up SleepOrder resources")
		cmd = exec.Command("kubectl", "delete", "sleeporder", "--all", "-A")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=sleepod-controller-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(MatchRegexp(`"msg":"Serving metrics server"`),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		Context("SleepOrder Controller", func() {
			It("should handle Deployment sleep/wake lifecycle", func() {
				By("creating a deployment")
				deploymentName := "test-deployment"
				deploymentYaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: 3
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: nginx
        image: nginx
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
`, deploymentName, namespace, deploymentName, deploymentName, deploymentName)

				verifySleepWakeLifecycle("Deployment", deploymentName, deploymentYaml, "deployment")
			})

			It("should handle StatefulSet sleep/wake lifecycle", func() {
				stsName := "test-sts"
				stsYaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    matchLabels:
      app: %s
  serviceName: %s
  replicas: 3
  template:
    metadata:
      labels:
        app: %s
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: nginx
        image: nginx
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
`, stsName, namespace, stsName, stsName, stsName)

				verifySleepWakeLifecycle("StatefulSet", stsName, stsYaml, "statefulset")
			})
		})

		Context("SleepPolicy Controller", func() {
			policyTestNamespace := "e2e-policy-test"

			BeforeEach(func() {
				cmd := exec.Command("kubectl", "create", "ns", policyTestNamespace)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				cmd := exec.Command("kubectl", "delete", "ns", policyTestNamespace)
				_, _ = utils.Run(cmd)
			})

			It("should create SleepOrder and handle lifecycle for Deployment via Policy", func() {
				By("creating a deployment")
				deploymentName := "policy-deployment"
				deploymentYaml := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: 3
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: nginx
        image: nginx
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
`, deploymentName, policyTestNamespace, deploymentName, deploymentName, deploymentName)

				tmpTargetFile, err := os.CreateTemp("", "policy-target-*.yaml")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.Remove(tmpTargetFile.Name()) }()
				_, err = tmpTargetFile.WriteString(deploymentYaml)
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpTargetFile.Close()).NotTo(HaveOccurred())

				cmd := exec.Command("kubectl", "apply", "-f", tmpTargetFile.Name())
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create Deployment for Policy test")

				By("creating a SleepPolicy")
				policyName := "e2e-policy"
				// Set sleep window: SleepAt 1 hour ago, WakeAt 1 hour from now -> Should be ASLEEP
				wakeAt := time.Now().UTC().Add(time.Hour).Format("15:04")
				sleepAt := time.Now().UTC().Add(-1 * time.Hour).Format("15:04")

				policyYaml := fmt.Sprintf(`
apiVersion: sleepod.sleepod.io/v1alpha1
kind: SleepPolicy
metadata:
  name: %s
  namespace: %s
spec:
  timezone: "UTC"
  deployments:
    %s:
      enable: true
      wakeAt: "%s"
      sleepAt: "%s"
`, policyName, policyTestNamespace, deploymentName, wakeAt, sleepAt)

				tmpPolicyFile, err := os.CreateTemp("", "policy-*.yaml")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.Remove(tmpPolicyFile.Name()) }()
				_, err = tmpPolicyFile.WriteString(policyYaml)
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpPolicyFile.Close()).NotTo(HaveOccurred())

				cmd = exec.Command("kubectl", "apply", "-f", tmpPolicyFile.Name())
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create SleepPolicy")

				// Expected SleepOrder Name: policyName-dep-resourceName
				expectedSleepOrderName := fmt.Sprintf("%s-dep-%s", policyName, deploymentName)

				By("verifying SleepOrder is created")
				verifySleepOrderCreated := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "sleeporder", expectedSleepOrderName, "-n", policyTestNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "SleepOrder should be created by Policy")
				}
				Eventually(verifySleepOrderCreated, 2*time.Minute, time.Second).Should(Succeed())

				By("verifying Deployment is scaled down (Sleep)")
				verifyScaledDown := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", policyTestNamespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("0"), "Deployment should be scaled down to 0")
				}
				Eventually(verifyScaledDown, 2*time.Minute, time.Second).Should(Succeed())

				By("updating SleepPolicy for wake window")
				// Set wake window: WakeAt 1 hour ago, SleepAt 1 hour from now -> Should be AWAKE
				wakeAt = time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
				sleepAt = time.Now().UTC().Add(time.Hour).Format("15:04")
				time.Sleep(5 * time.Second) // Small buffer

				cmd = exec.Command("kubectl", "patch", "sleeppolicy", policyName, "-n", policyTestNamespace, "--type=merge", "-p",
					fmt.Sprintf(`{"spec":{"deployments":{"%s":{"wakeAt":"%s","sleepAt":"%s"}}}}`, deploymentName, wakeAt, sleepAt))
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to update SleepPolicy")

				By("verifying Deployment is scaled up (Wake)")
				verifyScaledUp := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", namespace,
						"-o", "jsonpath={.spec.replicas}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("3"), "Deployment should be scaled up to 3")
				}
				Eventually(verifyScaledUp, 2*time.Minute, time.Second).Should(Succeed())
			})
		})
	})
})

func verifySleepWakeLifecycle(targetKind, targetName, targetYaml, kubectlGetCmd string) {
	By(fmt.Sprintf("creating a %s", targetKind))
	tmpTargetFile, err := os.CreateTemp("", "target-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(tmpTargetFile.Name()) }()
	_, err = tmpTargetFile.WriteString(targetYaml)
	Expect(err).NotTo(HaveOccurred())
	Expect(tmpTargetFile.Close()).NotTo(HaveOccurred())

	cmd := exec.Command("kubectl", "apply", "-f", tmpTargetFile.Name())
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create %s", targetKind))

	By("creating a SleepOrder for sleep window")
	sleepOrderName := fmt.Sprintf("test-sleeporder-%s", targetName)
	// Set sleep window: SleepAt 1 hour ago, WakeAt 1 hour from now -> Should be ASLEEP
	wakeAt := time.Now().UTC().Add(time.Hour).Format("15:04")
	sleepAt := time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
	sleepOrderYaml := fmt.Sprintf(`
apiVersion: sleepod.sleepod.io/v1alpha1
kind: SleepOrder
metadata:
  name: %s
  namespace: %s
spec:
  targetRef:
    kind: %s
    name: %s
  wakeAt: "%s"
  sleepAt: "%s"
  timezone: "UTC"
`, sleepOrderName, namespace, targetKind, targetName, wakeAt, sleepAt)

	tmpFile, err := os.CreateTemp("", "sleeporder-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, err = tmpFile.WriteString(sleepOrderYaml)
	Expect(err).NotTo(HaveOccurred())
	Expect(tmpFile.Close()).NotTo(HaveOccurred())

	cmd = exec.Command("kubectl", "apply", "-f", tmpFile.Name())
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create SleepOrder")

	By(fmt.Sprintf("verifying %s is scaled down (Sleep)", targetKind))
	verifyScaledDown := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", kubectlGetCmd, targetName, "-n", namespace,
			"-o", "jsonpath={.spec.replicas}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("0"), fmt.Sprintf("%s should be scaled down to 0", targetKind))
	}
	Eventually(verifyScaledDown, 2*time.Minute, time.Second).Should(Succeed())

	By("verifying SleepOrder status is Sleeping")
	verifyStatusSleeping := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "sleeporder", sleepOrderName, "-n", namespace,
			"-o", "jsonpath={.status.currentState}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("Sleeping"), "SleepOrder status should be Sleeping")
	}
	Eventually(verifyStatusSleeping, 2*time.Minute, time.Second).Should(Succeed())

	By("updating SleepOrder for wake window")
	// Set wake window: WakeAt 1 hour ago, SleepAt 1 hour from now -> Should be AWAKE
	wakeAt = time.Now().UTC().Add(-1 * time.Hour).Format("15:04")
	sleepAt = time.Now().UTC().Add(time.Hour).Format("15:04")
	time.Sleep(15 * time.Second)

	// Update the SleepOrder
	cmd = exec.Command("kubectl", "patch", "sleeporder", sleepOrderName, "-n", namespace, "--type=merge", "-p",
		fmt.Sprintf(`{"spec":{"wakeAt":"%s","sleepAt":"%s"}}`, wakeAt, sleepAt))
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to update SleepOrder")

	By(fmt.Sprintf("verifying %s is scaled up (Wake)", targetKind))
	verifyScaledUp := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", kubectlGetCmd, targetName, "-n", namespace,
			"-o", "jsonpath={.spec.replicas}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("3"), fmt.Sprintf("%s should be scaled up to 3", targetKind))
	}
	Eventually(verifyScaledUp, 2*time.Minute, time.Second).Should(Succeed())

	By("verifying SleepOrder status is Awake")
	verifyStatusAwake := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "sleeporder", sleepOrderName, "-n", namespace,
			"-o", "jsonpath={.status.currentState}")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("Awake"), "SleepOrder status should be Awake")
	}
	Eventually(verifyStatusAwake, 2*time.Minute, time.Second).Should(Succeed())
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

// helmNamespace where the project is deployed via helm
const helmNamespace = "sleepod-helm-test"

var _ = Describe("Helm Deployment", Ordered, func() {
	var chartPackagePath string

	BeforeAll(func() {
		By("packaging the helm chart")
		cmd := exec.Command("make", "helm-package")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to package helm chart")

		// Find the generated package
		matches, err := filepath.Glob("../../dist/sleepod-controller-*.tgz")
		Expect(err).NotTo(HaveOccurred())
		if len(matches) == 0 {
			// Try current dir if not found (relative path depends on execution dir)
			matches, err = filepath.Glob("dist/sleepod-controller-*.tgz")
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(matches).NotTo(BeEmpty(), "No helm package found in dist/")
		chartPackagePath = matches[0]

		// Ensure absolute path
		absPath, err := filepath.Abs(chartPackagePath)
		Expect(err).NotTo(HaveOccurred())
		chartPackagePath = absPath

		By("installing the helm chart")
		imageParts := strings.Split(projectImage, ":")
		Expect(imageParts).To(HaveLen(2), "Invalid projectImage format")
		repo := imageParts[0]
		tag := imageParts[1]

		By("creating a custom values.yaml")
		customValues := fmt.Sprintf(`
defaultNamespace: %s
controllerManager:
  container:
    image:
      repository: %s
      tag: %s
`, helmNamespace, repo, tag)
		customValuesPath := filepath.Join(filepath.Dir(chartPackagePath), "custom_values_e2e.yaml")
		err = os.WriteFile(customValuesPath, []byte(customValues), 0644)
		Expect(err).NotTo(HaveOccurred(), "Failed to create custom values file")

		cmd = exec.Command("helm", "install", "sleepod-controller", chartPackagePath,
			"--namespace", helmNamespace,
			"--create-namespace",
			"--values", customValuesPath,
		)
		_, _ = utils.Run(exec.Command("kubectl", "get", "namespaces"))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install helm chart")
	})

	AfterAll(func() {
		By("uninstalling the helm chart")
		cmd := exec.Command("helm", "uninstall", "sleepod-controller", "--namespace", helmNamespace)
		_, _ = utils.Run(cmd)

		By("deleting the helm namespace")
		cmd = exec.Command("kubectl", "delete", "ns", helmNamespace)
		_, _ = utils.Run(cmd)
	})

	Context("Helm Installation", func() {
		It("should run the controller successfully", func() {
			By("validating that the controller-manager pod is running")
			verifyControllerUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-n", helmNamespace,
					"-o", "jsonpath={.items[0].status.phase}",
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Controller pod is not running")
			}
			Eventually(verifyControllerUp, "2m", "1s").Should(Succeed())
		})
	})
})
