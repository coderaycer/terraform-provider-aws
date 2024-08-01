// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resiliencehub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resiliencehub"
	awstypes "github.com/aws/aws-sdk-go-v2/service/resiliencehub/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"

	// tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_resiliencehub_resiliency_policy", name="Resiliency Policy")
func newResourceResiliencyPolicy(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceResiliencyPolicy{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameResiliencyPolicy = "Resiliency Policy"
)

type resourceResiliencyPolicy struct {
	framework.ResourceWithConfigure
	framework.WithTimeouts
}

func (r *resourceResiliencyPolicy) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "aws_resiliencehub_resiliency_policy"
}

func (r *resourceResiliencyPolicy) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"policy_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexache.MustCompile("^[A-Za-z0-9][A-Za-z0-9_\\-]{1,59}$"), "Must match ^[A-Za-z0-9][A-Za-z0-9_\\-]{1,59}$"),
				},
			},
			names.AttrID: framework.IDAttribute(),
			"arn":        framework.ARNAttributeComputedOnly(),
			"policy_description": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtMost(500),
				},
			},
			"data_location_constraint": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.DataLocationConstraint](),
				Computed:   true,
				Optional:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tier": schema.StringAttribute{
				Required:   true,
				CustomType: fwtypes.StringEnumType[awstypes.ResiliencyPolicyTier](),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"estimated_cost_tier": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.EstimatedCostTier](),
				Computed:   true,
			},
			names.AttrTags: schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"policy": schema.SingleNestedBlock{
				CustomType: fwtypes.NewObjectTypeOf[resourceResiliencyPolicyModel](ctx),
				Validators: []validator.Object{
					objectvalidator.IsRequired(),
				},
				Blocks: map[string]schema.Block{
					"az": schema.SingleNestedBlock{
						CustomType: fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"region": schema.SingleNestedBlock{
						CustomType: fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"hardware": schema.SingleNestedBlock{
						CustomType: fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"software": schema.SingleNestedBlock{
						CustomType: fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
				},
			},
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *resourceResiliencyPolicy) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	conn := r.Meta().ResilienceHubClient(ctx)

	var plan resourceResiliencyPolicyDataModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &resiliencehub.CreateResiliencyPolicyInput{}
	resp.Diagnostics.Append(flex.Expand(ctx, &plan, in)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var policy map[string]awstypes.FailurePolicy
	if err := json.Unmarshal([]byte(plan.Policy.String()), &policy); err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionCreating, ResNameResiliencyPolicy, "", err),
			err.Error(),
		)
		return
	}

	in.Policy = setValidFailurePolicyKeys(policy)
	in.Tags = getTagsIn(ctx)

	out, err := conn.CreateResiliencyPolicy(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionCreating, ResNameResiliencyPolicy, "", err),
			err.Error(),
		)
		return
	}
	if out == nil || out.Policy == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionCreating, ResNameResiliencyPolicy, "", nil),
			errors.New("empty output").Error(),
		)
		return
	}

	plan.ARN = flex.StringToFramework(ctx, out.Policy.PolicyArn)
	plan.setId()

	resp.Diagnostics.Append(flex.Flatten(ctx, out.Policy, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceResiliencyPolicy) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {

	conn := r.Meta().ResilienceHubClient(ctx)

	var state resourceResiliencyPolicyDataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := findResiliencyPolicyByARN(ctx, conn, state.ARN.ValueString())
	if tfresource.NotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionSetting, ResNameResiliencyPolicy, state.ARN.ValueString(), err),
			err.Error(),
		)
		return
	}

	state.ARN = flex.StringToFramework(ctx, out.PolicyArn)
	state.ID = flex.StringToFramework(ctx, out.PolicyName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceResiliencyPolicy) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

	var old, new resourceResiliencyPolicyDataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.Plan.Get(ctx, &new)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResilienceHubClient(ctx)

	if !new.PolicyName.Equal(old.PolicyName) ||
		!new.ARN.Equal(old.ARN) {

		input := &resiliencehub.UpdateResiliencyPolicyInput{}
		resp.Diagnostics.Append(flex.Expand(ctx, new, input)...)
		if resp.Diagnostics.HasError() {
			return
		}

		_, err := conn.UpdateResiliencyPolicy(ctx, input)

		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("reading Resilience Hub policy name (%s)", new.PolicyName.String()), err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &new)...)
}

func (r *resourceResiliencyPolicy) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

	var state resourceResiliencyPolicyDataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResilienceHubClient(ctx)

	_, err := conn.DeleteResiliencyPolicy(ctx, &resiliencehub.DeleteResiliencyPolicyInput{
		PolicyArn: flex.StringFromFramework(ctx, state.ARN),
	})

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("deleting Resilience Hub policy name (%s)", state.PolicyName.String()), err.Error())

		return
	}
}

func (r *resourceResiliencyPolicy) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func findResiliencyPolicyByARN(ctx context.Context, conn *resiliencehub.Client, arn string) (*awstypes.ResiliencyPolicy, error) {
	in := &resiliencehub.DescribeResiliencyPolicyInput{
		PolicyArn: aws.String(arn),
	}

	out, err := conn.DescribeResiliencyPolicy(ctx, in)
	if err != nil {
		if errs.IsA[*awstypes.ResourceNotFoundException](err) {
			return nil, &retry.NotFoundError{
				LastError:   err,
				LastRequest: in,
			}
		}

		return nil, err
	}

	if out == nil || out.Policy == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out.Policy, nil
}

type resourceResiliencyPolicyDataModel struct {
	DataLocationConstraint fwtypes.StringEnum[awstypes.DataLocationConstraint]  `tfsdk:"data_location_constraint"`
	EstimatedCostTier      fwtypes.StringEnum[awstypes.EstimatedCostTier]       `tfsdk:"estimated_cost_tier"`
	ID                     types.String                                         `tfsdk:"id"`
	Policy                 fwtypes.ObjectValueOf[resourceResiliencyPolicyModel] `tfsdk:"policy"`
	ARN                    types.String                                         `tfsdk:"arn"`
	PolicyDescription      types.String                                         `tfsdk:"policy_description"`
	PolicyName             types.String                                         `tfsdk:"policy_name"`
	Tier                   fwtypes.StringEnum[awstypes.ResiliencyPolicyTier]    `tfsdk:"tier"`
	Tags                   types.Map                                            `tfsdk:"tags"`
	//TagsAll                types.Map                                                      `tfsdk:"tags_all"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

type resourceResiliencyPolicyModel struct {
	Region   fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"region"`
	AZ       fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"az"`
	Hardware fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"hardware"`
	Software fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"software"`
}

type resourceResiliencyObjectiveModel struct {
	RpoInSecs types.Int32 `tfsdk:"rpo_in_secs"`
	RtoInSecs types.Int32 `tfsdk:"rto_in_secs"`
}

func (r *resourceResiliencyPolicyDataModel) setId() {
	r.ID = r.PolicyName
}

func setValidFailurePolicyKeys(inPolicy map[string]awstypes.FailurePolicy) map[string]awstypes.FailurePolicy {
	outPolicy := make(map[string]awstypes.FailurePolicy)

	policyKeyMap := map[string]string{
		"az":       "AZ",
		"hardware": "Hardware",
		"region":   "Region",
		"software": "Software",
	}

	for key, policy := range inPolicy {
		if validKey, exists := policyKeyMap[key]; exists {
			outPolicy[validKey] = policy
		}
	}

	return outPolicy
}
