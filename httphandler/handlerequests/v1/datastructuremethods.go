package v1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/armosec/armoapi-go/armotypes"
	"github.com/kubescape/go-logger"
	"github.com/kubescape/go-logger/helpers"
	"github.com/kubescape/kubescape/v3/core/cautils"
	"github.com/kubescape/kubescape/v3/core/cautils/getter"
	apisv1 "github.com/kubescape/opa-utils/httpserver/apis/v1"
	utilsmetav1 "github.com/kubescape/opa-utils/httpserver/meta/v1"
	"k8s.io/utils/strings/slices"
)

func ToScanInfo(scanRequest *utilsmetav1.PostScanRequest) *cautils.ScanInfo {
	scanInfo := defaultScanInfo()

	setTargetInScanInfo(scanRequest, scanInfo)

	if scanRequest.Account != "" {
		scanInfo.AccountID = scanRequest.Account
	}
	if scanRequest.AccessKey != "" {
		scanInfo.AccessKey = scanRequest.AccessKey
	}
	if len(scanRequest.ExcludedNamespaces) > 0 {
		scanInfo.ExcludedNamespaces = strings.Join(scanRequest.ExcludedNamespaces, ",")
	}
	if len(scanRequest.IncludeNamespaces) > 0 {
		scanInfo.IncludeNamespaces = strings.Join(scanRequest.IncludeNamespaces, ",")
	}

	if scanRequest.Format != "" {
		scanInfo.Format = scanRequest.Format
	}

	// UseCachedArtifacts
	if scanRequest.UseCachedArtifacts != nil {
		if useCachedArtifacts := cautils.NewBoolPtr(scanRequest.UseCachedArtifacts); useCachedArtifacts.Get() != nil && *useCachedArtifacts.Get() {
			scanInfo.UseArtifactsFrom = getter.DefaultLocalStore // Load files from cache (this will prevent kubescape fom downloading the artifacts every time)
		}
	}

	// KeepLocal
	if scanRequest.KeepLocal != nil {
		if keepLocal := cautils.NewBoolPtr(scanRequest.KeepLocal); keepLocal.Get() != nil {
			scanInfo.Local = *keepLocal.Get() // Load files from cache (this will prevent kubescape fom downloading the artifacts every time)
		}
	}

	// submit
	if scanRequest.Submit != nil {
		if submit := cautils.NewBoolPtr(scanRequest.Submit); submit.Get() != nil {
			scanInfo.Submit = *submit.Get()
		}
	}

	// host scanner
	if scanRequest.HostScanner != nil {
		scanInfo.HostSensorEnabled = cautils.NewBoolPtr(scanRequest.HostScanner)
	}

	// single resource scan
	if scanRequest.ScanObject != nil {
		scanInfo.ScanObject = scanRequest.ScanObject
	}

	if scanRequest.IsDeletedScanObject != nil {
		scanInfo.IsDeletedScanObject = *scanRequest.IsDeletedScanObject
	}

	if scanRequest.Exceptions != nil {
		path, err := saveExceptions(scanRequest.Exceptions)
		if err != nil {
			logger.L().Warning("failed to save exceptions, scanning without them", helpers.Error(err))
		} else {
			scanInfo.UseExceptions = path
		}
	}

	return scanInfo
}

func setTargetInScanInfo(scanRequest *utilsmetav1.PostScanRequest, scanInfo *cautils.ScanInfo) {
	if scanRequest.TargetType != "" && len(scanRequest.TargetNames) > 0 {
		if strings.EqualFold(string(scanRequest.TargetType), string(apisv1.KindFramework)) {
			scanRequest.TargetType = apisv1.KindFramework
			scanInfo.FrameworkScan = true
			scanInfo.ScanAll = slices.Contains(scanRequest.TargetNames, "all") || slices.Contains(scanRequest.TargetNames, "")
			scanRequest.TargetNames = slices.Filter(nil, scanRequest.TargetNames, func(e string) bool { return e != "" && e != "all" })
		} else if strings.EqualFold(string(scanRequest.TargetType), string(apisv1.KindControl)) {
			scanRequest.TargetType = apisv1.KindControl
			scanInfo.ScanAll = false
		} else {
			// unknown policy kind - set scan all
			scanInfo.FrameworkScan = true
			scanInfo.ScanAll = true
			scanRequest.TargetNames = []string{}
		}
		scanInfo.SetPolicyIdentifiers(scanRequest.TargetNames, scanRequest.TargetType)
	} else {
		scanInfo.FrameworkScan = true
		scanInfo.ScanAll = true
	}
}

func saveExceptions(exceptions []armotypes.PostureExceptionPolicy) (string, error) {
	exceptionsJSON, err := json.Marshal(exceptions)
	if err != nil {
		return "", fmt.Errorf("failed to marshal exceptions: %w", err)
	}
	exceptionsPath := filepath.Join("/tmp", "exceptions.json") // FIXME potential race condition
	if err := os.WriteFile(exceptionsPath, exceptionsJSON, 0644); err != nil {
		return "", fmt.Errorf("failed to write exceptions file to disk: %w", err)
	}
	return exceptionsPath, nil
}
