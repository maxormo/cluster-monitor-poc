package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"os"
)

type Azure struct {
	account    ServicePrincipal
	authorizer autorest.Authorizer
}

func InitProvider(principal ServicePrincipal) Azure {
	initEnv(principal)
	var authorizer autorest.Authorizer

	var err error
	if authorizer, err = auth.NewAuthorizerFromEnvironment(); err != nil {
		panic(err)
	}
	azure := Azure{account: principal, authorizer: authorizer}
	return azure
}

func initEnv(creds ServicePrincipal) {
	os.Setenv("AZURE_TENANT_ID", creds.TenantId)
	os.Setenv("AZURE_CLIENT_ID", creds.AadId)
	os.Setenv("AZURE_CLIENT_SECRET", creds.AadSecret)
}

func (azure Azure) RestartNode(nodeName string) error {
	vmClient := azure.getVMClient()
	ctx := context.Background()
	future, err := vmClient.Restart(ctx, azure.account.ResourceGroup, nodeName)
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

func (azure Azure) getVMClient() compute.VirtualMachinesClient {
	vmClient := compute.NewVirtualMachinesClient(azure.account.Subscription)
	vmClient.Authorizer = azure.authorizer
	return vmClient
}
