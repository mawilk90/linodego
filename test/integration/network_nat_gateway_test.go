package integration

import (
	"context"
	"testing"

	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNATGateway(
	t *testing.T,
	fixtureYaml string,
	modifiers ...func(*linodego.NATGatewayCreateOptions),
) (*linodego.Client, *linodego.NATGateway, func(), error) {
	t.Helper()
	client, fixtureTeardown := createTestClient(t, fixtureYaml)
	createOpts := linodego.NATGatewayCreateOptions{
		Label:  "go-test-nat-gateway-" + randLabel(),
		Region: getRegionsWithCaps(t, client, []linodego.RegionCapability{linodego.CapabilityNATGateway})[0],
	}

	for _, modifier := range modifiers {
		modifier(&createOpts)
	}

	gateway, err := client.CreateNATGateway(context.Background(), createOpts)
	if err != nil {
		t.Fatalf("Error creating NAT Gateway: %v", err)
	}

	teardown := func() {
		if err := client.DeleteNATGateway(context.Background(), gateway.ID); err != nil {
			t.Fatalf("Error deleting NAT Gateway: %v", err)
		}
		fixtureTeardown()
	}
	return client, gateway, teardown, err
}

func verifyNATGateway(t *testing.T, gatewayCreated *linodego.NATGateway, gatewayRead *linodego.NATGateway) {
	assert.Equal(t, gatewayCreated.ID, gatewayRead.ID, "NAT Gateway ID mismatch")
	assert.Equal(t, gatewayCreated.Region, gatewayRead.Region, "NAT Gateway region mismatch")
	assert.Equal(t, gatewayCreated.Label, gatewayRead.Label, "NAT Gateway label mismatch")
	assert.Equal(t, 0, len(gatewayRead.Addresses), "Expected 0 addresses in NAT Gateway")
	assert.Greater(t, gatewayRead.AddressAutoscaleMax, 0, "Expected address autoscale max to be greater than 0 in NAT Gateway")
	assert.Equal(t, 4096, gatewayRead.DefaultPortsPerInterface, "Expected default ports per interface of 4096 in NAT Gateway")
	assert.Equal(t, 0, gatewayRead.PortsetAssignments, "Expected 0 portset assignment in NAT Gateway")
	assert.Greater(t, gatewayRead.PortsetCapacity, 0, "Expected portset capacity to be greater than 0 in NAT Gateway")
}

func TestNATGateway_Create_smoke(t *testing.T) {
	client, gatewayCreated, teardown, err := setupNATGateway(t, "fixtures/TestNATGateway_Create_smoke.yaml")
	defer teardown()
	require.NoErrorf(t, err, "Error creating NAT Gateway: %v", err)

	gateway, err := client.GetNATGateway(context.Background(), gatewayCreated.ID)
	require.NoErrorf(t, err, "Error retrieving NAT Gateway: %v", err)
	verifyNATGateway(t, gatewayCreated, gateway)

	f := linodego.Filter{}
	f.AddField(linodego.Eq, "label", gatewayCreated.Label)
	filter, err := f.MarshalJSON()
	require.NoErrorf(t, err, "Error marshalling filter: %v", err)

	gateways, err := client.ListNATGateways(context.Background(), &linodego.ListOptions{Filter: string(filter)})
	require.NoErrorf(t, err, "Error listing NAT Gateways: %v", err)
	require.Equal(t, 1, len(gateways), "Expected exactly one NAT Gateway in the list")
	verifyNATGateway(t, gatewayCreated, &gateways[0])
}
