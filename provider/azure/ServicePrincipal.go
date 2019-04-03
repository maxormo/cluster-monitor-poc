package azure

import (
	"encoding/json"
	"io/ioutil"
)

type ServicePrincipal struct {
	AadId, AadSecret, TenantId, Subscription, ResourceGroup string
}

func FromConfigFile(file string) ServicePrincipal {
	var values map[string]interface{}
	bytes, e := ioutil.ReadFile(file)

	if e != nil {
		panic(e)
	}

	e = json.Unmarshal(bytes, &values)

	if e != nil {
		panic(e)
	}

	return ServicePrincipal{
		Subscription:  values["subscriptionId"].(string),
		AadId:         values["aadClientId"].(string),
		AadSecret:     values["aadClientSecret"].(string),
		TenantId:      values["tenantId"].(string),
		ResourceGroup: values["resourceGroup"].(string)}
}
