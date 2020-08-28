package templates

const TerraformTemplate = `
######################
# S3 Terraform Backend
######################
terraform {
    backend "s3" {
        bucket = "<% .Config.Name %>"
        key    = "terraform.state"
        region = "<% .Config.Region %>"
    }
}

###################
# Variables
###################
variable "name" {
  description = "Name to be used on all AWS resources as identifier"
  default     = "<% .Config.Name %>"
}

variable "region" {
  description = "The AWS region to deploy"
  default     = "<% .Config.Region %>"
}

variable "device" {
    description = "Device type (marlin/sailfish)"
    default     = "<% .Config.Device %>"
}

variable "lambda_build_zip_file" {
    description = "Lambda build zip file"
    default     = "<% .LambdaZipFileLocation %>"
}

variable "shell_script_file" {
    description = "Shell script file"
    default     = "<% .BuildScriptFileLocation %>"
}

###################
# Provider
###################
provider "aws" {
    region = "${var.region}"
}

###################
# IAM
###################
resource "aws_iam_role" "rattlesnake_ec2_role" {
    name = "${var.name}-ec2"
    assume_role_policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
    {
        "Action": "sts:AssumeRole",
        "Principal": {
            "Service": "ec2.amazonaws.com"
        },
        "Effect": "Allow",
        "Sid": ""
    }
]
}
EOF
}
  
resource "aws_iam_role_policy" "rattlesnake_ec2_policy" {
    name = "${var.name}-ec2-policy"
    role = "${aws_iam_instance_profile.rattlesnake_ec2_role.id}"
    policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
    {
        "Effect": "Allow",
        "Action": [
            "sns:ListTopics",
            "sns:Publish"
        ],
        "Resource": "*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:GetObject",
            "s3:PutObject"
        ],
        "Resource": "arn:aws:s3:::${var.name}-keys/*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:ListBucket",
            "s3:GetBucketLocation"
        ],
        "Resource": "arn:aws:s3:::${var.name}-keys"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:GetObject",
            "s3:PutObject"
        ],
        "Resource": "arn:aws:s3:::${var.name}-keys-encrypted/*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:ListBucket",
            "s3:GetBucketLocation"
        ],
        "Resource": "arn:aws:s3:::${var.name}-keys-encrypted"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:GetObject",
            "s3:PutObject",
            "s3:ListMultipartUploadParts",
            "s3:AbortMultipartUpload"
        ],
        "Resource": "arn:aws:s3:::${var.name}-logs/*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:ListBucket",
            "s3:GetBucketLocation",
            "s3:ListBucketMultipartUploads"
        ],
        "Resource": "arn:aws:s3:::${var.name}-logs"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:GetObject",
            "s3:PutObject",
            "s3:PutObjectAcl",
            "s3:DeleteObject",
            "s3:ListMultipartUploadParts",
            "s3:AbortMultipartUpload"
        ],
        "Resource": "arn:aws:s3:::${var.name}-release/*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:ListBucket",
            "s3:GetBucketLocation",
            "s3:ListBucketMultipartUploads"
        ],
        "Resource": "arn:aws:s3:::${var.name}-release"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:GetObject"
        ],
        "Resource": "arn:aws:s3:::${var.name}-script/*"
    },
    {
        "Effect": "Allow",
        "Action": [
            "s3:ListBucket",
            "s3:GetBucketLocation"
        ],
        "Resource": "arn:aws:s3:::${var.name}-script"
    }
]
}
EOF
}

resource "aws_iam_instance_profile" "rattlesnake_ec2_role" {
    name = "${var.name}-ec2"
    role = "${aws_iam_role.rattlesnake_ec2_role.name}"
}

resource "aws_iam_role" "rattlesnake_lambda_role" {
  name = "${var.name}-lambda"
  assume_role_policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
    {
        "Action": "sts:AssumeRole",
        "Principal": {
            "Service": "lambda.amazonaws.com"
        },
        "Effect": "Allow",
        "Sid": ""
    }
]
}
EOF
}

resource "aws_iam_role_policy" "rattlesnake_lambda_policy" {
    name = "${var.name}-lambda-policy"
    role = "${aws_iam_role.rattlesnake_lambda_role.id}"
    policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
    {
        "Effect": "Allow",
        "Action": [
            "ec2:DescribeSubnets",
            "ec2:RequestSpotFleet",
            "ec2:DescribeSpotPriceHistory",
            "ec2:RunInstances",
            "ec2:CreateTags",
            "iam:CreateServiceLinkedRole",
            "iam:PassRole",
            "sts:GetCallerIdentity",
            "sns:Publish",
            "ec2:DescribeKeyPairs"
        ],
        "Resource": "*"
    }
]
}
EOF
}

###################
# S3
###################
resource "aws_s3_bucket" "rattlesnake_s3_keys" {
    bucket = "${var.name}-keys"
    force_destroy = true
  acl    = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}
resource "aws_s3_bucket" "rattlesnake_s3_keys_enc" {
    bucket = "${var.name}-keys-encrypted"
    force_destroy = true
  acl    = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}
resource "aws_s3_bucket" "rattlesnake_s3_logs" {
    bucket = "${var.name}-logs"
    force_destroy = true
  acl    = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}
resource "aws_s3_bucket" "rattlesnake_s3_release" {
    bucket = "${var.name}-release"
    force_destroy = true
  acl    = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}
resource "aws_s3_bucket" "rattlesnake_s3_script" {
    bucket = "${var.name}-script"
    force_destroy = true
  acl    = "private"

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "aws_s3_bucket_object" "rattlesnake_s3_script_file" {
  bucket = "${var.name}-script"
  key    = "build.sh"
  source = "${var.shell_script_file}"
  etag   = "${md5(file("${var.shell_script_file}"))}"

  depends_on = ["aws_s3_bucket.rattlesnake_s3_script"]
}

###################
# SNS
###################
resource "aws_sns_topic" "rattlesnake" {
  name = "${var.name}"
}

###################
# Lambda
###################
resource "aws_lambda_function" "rattlesnake_lambda_build" {
    filename         = "${var.lambda_build_zip_file}"
    function_name    = "${var.name}-build"
    role             = "${aws_iam_role.rattlesnake_lambda_role.arn}"
    handler          = "lambda_spot_function.lambda_handler"
    source_code_hash = "${base64sha256(file("${var.lambda_build_zip_file}"))}"
    runtime          = "python3.6"
    timeout          = "60"
}

###################
# Cloudwatch Event
###################
resource "aws_cloudwatch_event_rule" "build_schedule" {
    name = "${var.name}-build-schedule"
    description = "RattlesnakeOS build"
    schedule_expression = "<% .Config.Schedule %>"
}

resource "aws_cloudwatch_event_target" "check_build_schedule" {
    rule = "${aws_cloudwatch_event_rule.build_schedule.name}"
    target_id = "${var.name}-build"
    arn = "${aws_lambda_function.rattlesnake_lambda_build.arn}"
}

resource "aws_lambda_permission" "allow_cloudwatch_to_call_build_schedule" {
    statement_id = "AllowExecutionFromCloudWatch"
    action = "lambda:InvokeFunction"
    function_name = "${aws_lambda_function.rattlesnake_lambda_build.function_name}"
    principal = "events.amazonaws.com"
    source_arn = "${aws_cloudwatch_event_rule.build_schedule.arn}"
}

###################
# Outputs
###################
output "sns_topic_arn" {
    description = "The SNS ARN"
    value = "${aws_sns_topic.rattlesnake.arn}"
}
`
