// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package meta

import (
	"context"
	"fmt"

	"github.com/hashicorp/aws-sdk-go-base/v2/endpoints"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkDataSource(name="Service Principal")
func newServicePrincipalDataSource(context.Context) (datasource.DataSourceWithConfigure, error) {
	d := &servicePrincipalDataSource{}

	return d, nil
}

type servicePrincipalDataSource struct {
	framework.DataSourceWithConfigure
}

func (*servicePrincipalDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) { // nosemgrep:ci.meta-in-func-name
	response.TypeName = "aws_service_principal"
}

func (d *servicePrincipalDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrID: schema.StringAttribute{
				Computed: true,
			},
			names.AttrName: schema.StringAttribute{
				Computed: true,
			},
			names.AttrRegion: schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			names.AttrServiceName: schema.StringAttribute{
				Required: true,
			},
			"suffix": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *servicePrincipalDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var data servicePrincipalDataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	var region *endpoints.Region

	// find the region given by the user
	if !data.Region.IsNull() {
		name := data.Region.ValueString()
		matchingRegion, err := findRegionByName(ctx, name)

		if err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("finding Region by name (%s)", name), err.Error())

			return
		}

		region = matchingRegion
	}

	// Default to provider current Region if no other filters matched.
	if region == nil {
		name := d.Meta().Region
		matchingRegion, err := findRegionByName(ctx, name)

		if err != nil {
			response.Diagnostics.AddError(fmt.Sprintf("finding Region by name (%s)", name), err.Error())

			return
		}

		region = matchingRegion
	}

	partition := names.PartitionForRegion(region.ID())

	serviceName := ""

	if !data.ServiceName.IsNull() {
		serviceName = data.ServiceName.ValueString()
	}

	sourceServicePrincipal := names.ServicePrincipalNameForPartition(serviceName, partition)

	data.ID = fwflex.StringValueToFrameworkLegacy(ctx, serviceName+"."+region.ID()+"."+sourceServicePrincipal)
	data.Name = fwflex.StringValueToFrameworkLegacy(ctx, serviceName+"."+sourceServicePrincipal)
	data.Suffix = fwflex.StringValueToFrameworkLegacy(ctx, sourceServicePrincipal)
	data.Region = fwflex.StringValueToFrameworkLegacy(ctx, region.ID())

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

type servicePrincipalDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Region      types.String `tfsdk:"region"`
	ServiceName types.String `tfsdk:"service_name"`
	Suffix      types.String `tfsdk:"suffix"`
}
