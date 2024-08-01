provider "aws" {
  region  = "us-east-1"
  profile = "hgreen-sb-terraform"
}


resource "aws_resiliencehub_resiliency_policy" "test_policy" {

  policy_name        = "test"
  policy_description = "tester"

  tier = "NonCritical"

  policy {
    region {
      rpo_in_secs = 3600
      rto_in_secs = 3600
    }
    az {
      rpo_in_secs = 3600
      rto_in_secs = 3600
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
