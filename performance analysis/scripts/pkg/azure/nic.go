package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func (c *Client) CreateNIC(ctx context.Context, name, resourceGroup, subnetID string, publicIpID *string) (*armnetwork.Interface, error) {
	var publicIP *armnetwork.PublicIPAddress
	if publicIpID != nil {
		publicIP = &armnetwork.PublicIPAddress{
			ID: publicIpID,
		}
	}

	pResp, err := c.InterfacesClient.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.Interface{
		Location: to.Ptr(c.Location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(subnetID),
						},
						PublicIPAddress: publicIP,
					},
				},
			},
		},
	}, nil)

	if err != nil {
		return nil, err
	}

	resp, err := pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.Interface, nil
}

func (c *Client) DeleteNIC(ctx context.Context, name, resourceGroup string) error {
	pResp, err := c.InterfacesClient.BeginDelete(ctx, resourceGroup, name, nil)

	_, err = pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
