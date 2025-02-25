package mco

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/openshift-kni/eco-goinfra/pkg/clients"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func ListMCP(apiClient *clients.Settings, listOptions metav1.ListOptions) ([]*MCPBuilder, error) {
	glog.V(100).Infof("Listing all MCP resources with the options %v", listOptions)

	mcpList, err := apiClient.MachineConfigPools().List(context.Background(), listOptions)

	if err != nil {
		glog.V(100).Infof("Failed to list MCP objects due to %s", err.Error())

		return nil, err
	}

	var mcpObjects []*MCPBuilder

	for _, mcp := range mcpList.Items {
		copiedMcp := mcp
		mcpBuilder := &MCPBuilder{
			apiClient:  apiClient,
			Object:     &copiedMcp,
			Definition: &copiedMcp,
		}

		mcpObjects = append(mcpObjects, mcpBuilder)
	}

	return mcpObjects, nil
}

func ListMCPByMachineConfigSelector(apiClient *clients.Settings, mcpLabel string) (*MCPBuilder, error) {
	glog.V(100).Infof("GetByLabel returns MachineConfigPool with the specified label: %v", mcpLabel)

	mcpList, err := ListMCP(apiClient, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	for _, mcp := range mcpList {
		for _, label := range mcp.Object.Spec.MachineConfigSelector.MatchExpressions {
			for _, value := range label.Values {
				if value == mcpLabel {
					return mcp, nil
				}
			}
		}

		for _, label := range mcp.Object.Spec.MachineConfigSelector.MatchLabels {
			if label == mcpLabel {
				return mcp, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot find MachineConfigPool that targets machineConfig with label: %s", mcpLabel)
}

func ListMCPWaitToBeStableFor(apiClient *clients.Settings, stableDuration, timeout time.Duration) error {
	glog.V(100).Infof("WaitForMcpListToBeStableFor waits up to duration of %v for "+
		"MachineConfigPoolList to be stable for %v", timeout, stableDuration)

	isMcpListStable := true

	// Wait 5 secs in each iteration before condition function () returns true or errors or times out
	// after stableDuration
	err := wait.PollImmediate(fiveScds, timeout, func() (bool, error) {

		isMcpListStable = true

		// check if cluster is stable every 5 seconds during entire stableDuration time period
		// Here we need to run through the entire stableDuration till it times out.
		_ = wait.PollImmediate(fiveScds, stableDuration, func() (done bool, err error) {

			mcpList, err := ListMCP(apiClient, metav1.ListOptions{})

			if err != nil {
				return false, err
			}

			// iterate through the MachineConfigPools in the list.
			for _, mcp := range mcpList {
				if mcp.Object.Status.ReadyMachineCount != mcp.Object.Status.MachineCount ||
					mcp.Object.Status.MachineCount != mcp.Object.Status.UpdatedMachineCount ||
					mcp.Object.Status.DegradedMachineCount != 0 {
					isMcpListStable = false

					glog.V(100).Infof("MachineConfigPool: %v degraded and has a mismatch in "+
						"machineCount: %v "+"vs machineCountUpdated: "+"%v vs readyMachineCount: %v and "+
						"degradedMachineCount is : %v \n", mcp.Object.ObjectMeta.Name,
						mcp.Object.Status.MachineCount, mcp.Object.Status.UpdatedMachineCount,
						mcp.Object.Status.ReadyMachineCount, mcp.Object.Status.DegradedMachineCount)

					return true, err
				}
			}

			// Here we are always returning "false, nil" so we keep iterating throughout the stableInterval
			// of the inner wait.pollImmediate loop, until we time out.
			return false, nil
		})

		if isMcpListStable {
			glog.V(100).Infof("MachineConfigPools were stable during during stableDuration: %v",
				stableDuration)

			// exit the outer wait.PollImmediate block since the mcps were stable during stableDuration.
			return true, nil
		}

		glog.V(100).Infof("MachineConfigPools were not stable during stableDuration: %v, retrying ...",
			stableDuration)

		// keep iterating in the outer wait.PollImmediate waiting for cluster to be stable.
		return false, nil

	})

	if err == nil {
		glog.V(100).Infof("Cluster was stable during stableDuration: %v", stableDuration)
	} else {
		// Here err is "timed out waiting for the condition"
		glog.V(100).Infof("Cluster was Un-stable during stableDuration: %v", stableDuration)
	}

	return err
}
