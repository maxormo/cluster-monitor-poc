package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"os"
	"strings"
)

type azure struct {
	account    ServicePrincipal
	authorizer autorest.Authorizer
}

func InitProvider(principal ServicePrincipal) azure {
	initEnv(principal)
	var authorizer autorest.Authorizer

	var err error
	if authorizer, err = auth.NewAuthorizerFromEnvironment(); err != nil {
		panic(err)
	}
	azure := azure{account: principal, authorizer: authorizer}
	return azure
}

func initEnv(creds ServicePrincipal) {
	os.Setenv("AZURE_TENANT_ID", creds.TenantId)
	os.Setenv("AZURE_CLIENT_ID", creds.AadId)
	os.Setenv("AZURE_CLIENT_SECRET", creds.AadSecret)
}

func (azure azure) RestartNode(nodeName string) error {
	vmClient := azure.getVMClient()
	ctx := context.Background()
	nodeId, err := azure.findNodeId(vmClient, nodeName)
	if err != nil {
		return fmt.Errorf("cannot find vm id : %v", err)
	}

	future, err := vmClient.Restart(ctx, azure.account.ResourceGroup, azure.account.ScaleSetName, nodeId)
	if err != nil {
		return fmt.Errorf("cannot restart vm: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, vmClient.Client)
	if err != nil {
		return fmt.Errorf("cannot get the vm restart future response: %v", err)
	}

	_, err = future.Result(vmClient)
	return err
}

func (azure azure) findNodeId(client compute.VirtualMachineScaleSetVMsClient, nodeName string) (string, error) {
	ctx := context.Background()
	result, err := client.List(ctx, azure.account.ResourceGroup, azure.account.ScaleSetName, "", "", "")
	if err != nil {
		return "", err
	}

	for _, value := range result.Values() {
		if strings.EqualFold(nodeName, *value.OsProfile.ComputerName) {
			return *value.InstanceID, nil
		}
	}
	return "", fmt.Errorf("cannot find %s in vmss %s", nodeName, azure.account.ScaleSetName)

}

func (azure azure) getVMClient() compute.VirtualMachineScaleSetVMsClient {

	vmClient := compute.NewVirtualMachineScaleSetVMsClient(azure.account.Subscription)
	vmClient.Authorizer = azure.authorizer
	return vmClient
}
