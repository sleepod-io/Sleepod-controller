package controller

// Global consts:
const (
	defaultTimezone string = "UTC"
	kindDeployment  string = "Deployment"
	kindStatefulSet string = "StatefulSet"
)

// SleepOrder Annotations:
const (
	configHashAnnotationKey       string = "sleepod.io/configHash"
	originalReplicasAnnotationKey string = "sleepod.io/original-replicas"
	sleepOrderFinalizer           string = "sleepod.io/finalizer"
)

// SleepOrder Status:
const (
	sleepingState string = "Sleeping"
	awakeState    string = "Awake"
)

// sleepPolicy actions:
const (
	actionCreate string = "create"
	actionUpdate string = "update"
)
