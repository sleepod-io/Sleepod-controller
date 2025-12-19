package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	sleepodv1alpha1 "github.com/shaygef123/SleePod-controller/api/v1alpha1"
)

func hashCalc(input string) string {
	md5Hash := md5.Sum([]byte(input))
	return hex.EncodeToString(md5Hash[:6])
}

func ConfigHashCalc(config sleepodv1alpha1.ResourceSleepParams) string {
	configStr := fmt.Sprintf("%v", config)
	return hashCalc(configStr)
}
