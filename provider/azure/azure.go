package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
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

func (azure azure) ListVmss() []string {

	client := resources.NewClient(azure.account.Subscription)
	client.Authorizer = azure.authorizer

	ctx := context.Background()
	page, e := client.ListByResourceGroup(ctx, azure.account.ResourceGroup, "resourceType eq 'Microsoft.Compute/virtualMachineScaleSets'", "", nil)

	if e != nil {
		panic(e)
	}
	var result []string
	for e == nil && len(page.Values()) != 0 {
		for _, v := range page.Values() {
			result = append(result, *v.Name)
		}
		e = page.NextWithContext(ctx)
	}
	return result
}

func (azure azure) RestartNode(nodeName string) error {
	vmClient := azure.getVMClient()
	ctx := context.Background()
	vmsss := azure.ListVmss()
	var nodeId string
	var err error
	var targetVmss string
	for _, vmss := range vmsss {
		targetVmss = vmss
		nodeId, err = azure.findNodeId(vmClient, nodeName, targetVmss)
		if err == nil {
			break
		}
	}
	if nodeId == "" {
		return fmt.Errorf("cannot find vm id : %v", err)
	}

	future, err := vmClient.Restart(ctx, azure.account.ResourceGroup, targetVmss, nodeId)
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

func (azure azure) findNodeId(client compute.VirtualMachineScaleSetVMsClient, nodeName string, scaleSetName string) (string, error) {
	ctx := context.Background()

	result, err := client.List(ctx, azure.account.ResourceGroup, scaleSetName, "", "", "")
	if err != nil {
		return "", err
	}

	for _, value := range result.Values() {
		if strings.EqualFold(nodeName, *value.OsProfile.ComputerName) {
			return *value.InstanceID, nil
		}
	}
	return "", fmt.Errorf("cannot find %s in vmss %s", nodeName, scaleSetName)

}

func (azure azure) getVMClient() compute.VirtualMachineScaleSetVMsClient {

	vmClient := compute.NewVirtualMachineScaleSetVMsClient(azure.account.Subscription)
	vmClient.Authorizer = azure.authorizer
	return vmClient
}
