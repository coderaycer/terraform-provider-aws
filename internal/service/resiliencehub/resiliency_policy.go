// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resiliencehub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
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
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
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
	framework.WithImportByID
	framework.WithTimeouts
}

func (r *resourceResiliencyPolicy) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "aws_resiliencehub_resiliency_policy"
}

func (r *resourceResiliencyPolicy) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrID: framework.IDAttribute(),
			"arn":        framework.ARNAttributeComputedOnly(),
			"policy_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexache.MustCompile("^[A-Za-z0-9][A-Za-z0-9_\\-]{1,59}$"), "Must match ^[A-Za-z0-9][A-Za-z0-9_\\-]{1,59}$"),
				},
			},
			"policy_description": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tier": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.ResiliencyPolicyTier](),
				Required:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"estimated_cost_tier": schema.StringAttribute{
				CustomType: fwtypes.StringEnumType[awstypes.EstimatedCostTier](),
				Computed:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			names.AttrTags: tftags.TagsAttribute(),
		},
		Blocks: map[string]schema.Block{
			"policy": schema.SingleNestedBlock{
				CustomType:  fwtypes.NewObjectTypeOf[resourceResiliencyComponentModel](ctx),
				Description: "AWS Resilience Hub resiliency failure policy.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Validators: []validator.Object{
					objectvalidator.IsRequired(),
				},
				Blocks: map[string]schema.Block{
					"az": schema.SingleNestedBlock{
						CustomType:  fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Description: "AWS Resilience Hub RTO and RPO target to measure resiliency for potential availability zone disruptions.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.RequiresReplace(),
						},
						Validators: []validator.Object{
							objectvalidator.IsRequired(),
						},
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"hardware": schema.SingleNestedBlock{
						CustomType:  fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Description: "AWS Resilience Hub RTO and RPO target to measure resiliency for potential infrastructure disruptions.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.RequiresReplace(),
						},
						Validators: []validator.Object{
							objectvalidator.IsRequired(),
						},
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"software": schema.SingleNestedBlock{
						CustomType:  fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Description: "AWS Resilience Hub RTO and RPO target to measure resiliency for potential application disruptions.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.RequiresReplace(),
						},
						Validators: []validator.Object{
							objectvalidator.IsRequired(),
						},
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
						},
					},
					"region": schema.SingleNestedBlock{
						CustomType:  fwtypes.NewObjectTypeOf[resourceResiliencyObjectiveModel](ctx),
						Description: "AWS Resilience Hub RTO and RPO target to measure resiliency for potential region disruptions.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.RequiresReplace(),
						},
						Attributes: map[string]schema.Attribute{
							"rto_in_secs": schema.Int32Attribute{
								Description: "Recovery Time Objective (RTO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
									int32planmodifier.RequiresReplace(),
								},
							},
							"rpo_in_secs": schema.Int32Attribute{
								Description: "Recovery Point Objective (RPO) in seconds.",
								Required:    true,
								PlanModifiers: []planmodifier.Int32{
									int32planmodifier.UseStateForUnknown(),
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

	var plan resourceResiliencyPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &resiliencehub.CreateResiliencyPolicyInput{}
	resp.Diagnostics.Append(flex.Expand(ctx, &plan, in)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in.Policy = failurePolicyFromFramework(plan.Policy)
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

	createTimeout := r.CreateTimeout(ctx, plan.Timeouts)
	_, err = waitResiliencyPolicyCreated(ctx, conn, plan.ID.ValueString(), createTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionWaitingForCreation, ResNameResiliencyPolicy, plan.ID.ValueString(), err),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *resourceResiliencyPolicy) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	conn := r.Meta().ResilienceHubClient(ctx)

	var state resourceResiliencyPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := findResiliencyPolicyByID(ctx, conn, state.ID.ValueString())
	if tfresource.NotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionSetting, ResNameResiliencyPolicy, state.ID.ValueString(), err),
			err.Error(),
		)
		return
	}

	state.ARN = flex.StringToFramework(ctx, policy.PolicyArn)
	state.ID = flex.StringToFramework(ctx, policy.PolicyArn)

	resp.Diagnostics.Append(flex.Flatten(ctx, policy, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceResiliencyPolicy) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

	var plan, state resourceResiliencyPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResilienceHubClient(ctx)

	if !plan.PolicyDescription.Equal(state.PolicyDescription) ||
		!plan.DataLocationConstraint.Equal(state.DataLocationConstraint) ||
		!plan.Tier.Equal(state.Tier) {

		planPolicyId := flex.StringFromFramework(ctx, plan.ID)

		in := &resiliencehub.UpdateResiliencyPolicyInput{
			PolicyArn: planPolicyId,
		}

		if !plan.PolicyDescription.Equal(state.PolicyDescription) {
			in.PolicyDescription = flex.StringFromFramework(ctx, plan.PolicyDescription)
		}

		if !plan.DataLocationConstraint.Equal(state.DataLocationConstraint) {
			in.DataLocationConstraint = plan.DataLocationConstraint.ValueEnum()
		}

		if !plan.Tier.Equal(state.Tier) {
			in.Tier = plan.Tier.ValueEnum()
		}

		_, err := conn.UpdateResiliencyPolicy(ctx, in)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("reading Resilience Hub policy ID (%s)", *planPolicyId), err.Error())
			return
		}

		policy, err := findResiliencyPolicyByID(ctx, conn, *planPolicyId)
		if tfresource.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		if err != nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionSetting, ResNameResiliencyPolicy, *planPolicyId, err),
				err.Error(),
			)
			return
		}

		updateTimeout := r.UpdateTimeout(ctx, plan.Timeouts)
		_, err = waitResiliencyPolicyUpdated(ctx, conn, plan.ID.ValueString(), updateTimeout)
		if err != nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionWaitingForUpdate, ResNameResiliencyPolicy, plan.ID.String(), err),
				err.Error(),
			)
			return
		}

		resp.Diagnostics.Append(flex.Flatten(ctx, policy, &plan)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceResiliencyPolicy) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

	var state resourceResiliencyPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().ResilienceHubClient(ctx)

	_, err := conn.DeleteResiliencyPolicy(ctx, &resiliencehub.DeleteResiliencyPolicyInput{
		PolicyArn: flex.StringFromFramework(ctx, state.ID),
	})

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("deleting Resilience Hub policy name (%s)", state.PolicyName.String()), err.Error())
		return
	}

	deleteTimeout := r.DeleteTimeout(ctx, state.Timeouts)
	_, err = waitResiliencyPolicyDeleted(ctx, conn, state.ID.ValueString(), deleteTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.ResilienceHub, create.ErrActionWaitingForDeletion, ResNameResiliencyPolicy, state.ID.String(), err),
			err.Error(),
		)
		return
	}
}

func (r *resourceResiliencyPolicy) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(names.AttrID), req.ID)...)
}

const (
	statusChangePending = "Pending"
	statusDeleting      = "Deleting"
	statusNormal        = "Normal"
	statusUpdated       = "Updated"
	statusCompleted     = "Completed"
)

func waitResiliencyPolicyCreated(ctx context.Context, conn *resiliencehub.Client, id string, timeout time.Duration) (*awstypes.ResiliencyPolicy, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   []string{},
		Target:                    []string{statusCompleted},
		Refresh:                   statusResiliencyPolicy(ctx, conn, id),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*awstypes.ResiliencyPolicy); ok {
		return out, err
	}

	return nil, err
}

func waitResiliencyPolicyUpdated(ctx context.Context, conn *resiliencehub.Client, id string, timeout time.Duration) (*awstypes.ResiliencyPolicy, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   []string{},
		Target:                    []string{statusCompleted},
		Refresh:                   statusResiliencyPolicy(ctx, conn, id),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*awstypes.ResiliencyPolicy); ok {
		return out, err
	}

	return nil, err
}

func waitResiliencyPolicyDeleted(ctx context.Context, conn *resiliencehub.Client, id string, timeout time.Duration) (*awstypes.ResiliencyPolicy, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{},
		Target:  []string{},
		Refresh: statusResiliencyPolicy(ctx, conn, id),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*awstypes.ResiliencyPolicy); ok {
		return out, err
	}

	return nil, err
}

func statusResiliencyPolicy(ctx context.Context, conn *resiliencehub.Client, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		out, err := findResiliencyPolicyByID(ctx, conn, id)
		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return out, statusCompleted, nil
	}
}

func findResiliencyPolicyByID(ctx context.Context, conn *resiliencehub.Client, id string) (*awstypes.ResiliencyPolicy, error) {
	in := &resiliencehub.DescribeResiliencyPolicyInput{
		PolicyArn: aws.String(id),
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

func (r *resourceResiliencyPolicyModel) setId() {
	r.ID = r.ARN
}

// failurePolicyFromFramework function modifies FailurePolicy keys to align with CreateResiliencyPolicy
// FailurePolicy valid key values.
// See https://docs.aws.amazon.com/resilience-hub/latest/APIReference/API_CreateResiliencyPolicy.html
func failurePolicyFromFramework(planPolicy fwtypes.ObjectValueOf[resourceResiliencyComponentModel]) map[string]awstypes.FailurePolicy {
	inPolicy := make(map[string]awstypes.FailurePolicy)

	policyKeyMap := map[string]string{
		"az":       "AZ",
		"hardware": "Hardware",
		"region":   "Region",
		"software": "Software",
	}

	for attr, value := range planPolicy.Attributes() {
		if validKey, exists := policyKeyMap[attr]; exists {
			var policy awstypes.FailurePolicy
			var failurePolicy map[string]any
			if err := json.Unmarshal([]byte(value.String()), &failurePolicy); err == nil {
				if rpoInSecs, ok := failurePolicy["rpo_in_secs"].(float64); ok {
					policy.RpoInSecs = int32(rpoInSecs)
				}
				if rtoInSecs, ok := failurePolicy["rto_in_secs"].(float64); ok {
					policy.RtoInSecs = int32(rtoInSecs)
				}
			}
			inPolicy[validKey] = policy
		}
	}

	return inPolicy
}

func getFailurePolicy(planPolicy fwtypes.ObjectValueOf[resourceResiliencyComponentModel]) []string {

	var values []string

	policyKeyMap := map[string]string{
		"az":       "AZ",
		"hardware": "Hardware",
		"region":   "Region",
		"software": "Software",
	}

	for attr, value := range planPolicy.Attributes() {
		if _, exists := policyKeyMap[attr]; exists {
			var obj map[string]any
			// var policy awstypes.FailurePolicy
			if err := json.Unmarshal([]byte(value.String()), &obj); err == nil {
				rpoInSecs := fmt.Sprintf("%s", reflect.TypeOf(obj["rpo_in_secs"]))
				values = append(values, rpoInSecs)
			}
		}
	}

	return values
}

type resourceResiliencyPolicyModel struct {
	DataLocationConstraint fwtypes.StringEnum[awstypes.DataLocationConstraint]     `tfsdk:"data_location_constraint"`
	EstimatedCostTier      fwtypes.StringEnum[awstypes.EstimatedCostTier]          `tfsdk:"estimated_cost_tier"`
	ID                     types.String                                            `tfsdk:"id"`
	Policy                 fwtypes.ObjectValueOf[resourceResiliencyComponentModel] `tfsdk:"policy"`
	ARN                    types.String                                            `tfsdk:"arn"`
	PolicyDescription      types.String                                            `tfsdk:"policy_description"`
	PolicyName             types.String                                            `tfsdk:"policy_name"`
	Tier                   fwtypes.StringEnum[awstypes.ResiliencyPolicyTier]       `tfsdk:"tier"`
	Tags                   types.Map                                               `tfsdk:"tags"`
	Timeouts               timeouts.Value                                          `tfsdk:"timeouts"`
}

type resourceResiliencyComponentModel struct {
	Region   fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"region"`
	AZ       fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"az"`
	Hardware fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"hardware"`
	Software fwtypes.ObjectValueOf[resourceResiliencyObjectiveModel] `tfsdk:"software"`
}

type resourceResiliencyObjectiveModel struct {
	RpoInSecs types.Int32 `tfsdk:"rpo_in_secs"`
	RtoInSecs types.Int32 `tfsdk:"rto_in_secs"`
}
