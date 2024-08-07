terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
    awscc = {
      source = "hashicorp/awscc"
    }
  }
}


provider "aws" {
  region  = "us-east-1"
  profile = "hgreen-sb-terraform"
}

# provider "awscc" {
#   region  = "us-east-1"
#   profile = "hgreen-sb-terraform"
# }

# resource "awscc_resiliencehub_resiliency_policy" "awscc_test_policy" {

#   policy_name        = "awscc-test"
#   policy_description = "tester"

#   tier = "NonCritical"

#   policy = {
#     region = {
#       rpo_in_secs = 3
#       rto_in_secs = 3
#     }
#     az = {
#       rpo_in_secs = 3600
#       rto_in_secs = 3600
#     }
#     hardware = {
#       rpo_in_secs = 3600
#       rto_in_secs = 3600
#     }
#     software = {
#       rpo_in_secs = 3600
#       rto_in_secs = 3600
#     }
#   }

#   tags = {
#     appId = "test"
#   }
# }

# import {
#   to = aws_resiliencehub_resiliency_policy.aws_test_policy
#   id = "arn:aws:resiliencehub:us-east-1:253131516168:resiliency-policy/21d4d0f9-4a98-47d2-bc7f-9093377966af"
# }

resource "aws_resiliencehub_resiliency_policy" "aws_test_policy" {

  policy_name        = "aws-test"
  policy_description = "aws-tester"

  tier = "NonCritical"

  data_location_constraint = "AnyLocation"

  policy {
    region {
      rpo_in_secs = 3600
      rto_in_secs = 3600
    }
    az {
      rpo_in_secs = 86400
      rto_in_secs = 86400
    }
    hardware {
      rpo_in_secs = 3600
      rto_in_secs = 3600
    }
    software {
      rpo_in_secs = 3600
      rto_in_secs = 3600
    }
  }

  tags = {
    appId = "test"
  }
}
