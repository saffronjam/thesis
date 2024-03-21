package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func (c *Client) CreatePublicIP(ctx context.Context, name, resourceGroup string) (*armnetwork.PublicIPAddress, error) {
	pResp, err := c.PublicIpAddressesClient.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.PublicIPAddress{
		Location: to.Ptr(c.Location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)

	if err != nil {
		return nil, err
	}

	resp, err := pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.PublicIPAddress, nil
}

func (c *Client) DeletePublicIP(ctx context.Context, name, resourceGroup string) error {
	pResp, err := c.PublicIpAddressesClient.BeginDelete(ctx, resourceGroup, name, nil)

	_, err = pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
