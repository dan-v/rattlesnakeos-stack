package templates

const TerraformTemplate = `
######################
# S3 Terraform Backend
######################
terraform {
	backend "s3" {
		bucket = "<% .Name %>"
		key    = "terraform.state"
		region = "<% .Region %>"
	}
}

###################
# Variables
###################
variable "name" {
  description = "Name to be used on all AWS resources as identifier"
  default     = "<% .Name %>"
}

variable "region" {
  description = "The AWS region to deploy"
  default     = "<% .Region %>"
}

variable "device" {
	description = "Device type (marlin/sailfish)"
	default     = "<% .Device %>"
}

variable "lambda_build_zip_file" {
	description = "Lambda build zip file"
	default     = "<% .LambdaSpotZipFile %>"
}

variable "shell_script_file" {
	description = "Shell script file"
	default     = "<% .ShellScriptFile %>"
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
		"Action": "*",
		"Resource": "*"
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
		"Action": "*",
		"Resource": "*"
	}
]
}
EOF
}

resource "aws_iam_role" "rattlesnake_spot_fleet_role" {
	name = "${var.name}-spot-fleet-role"
	assume_role_policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
	{
		"Sid": "",
		"Effect": "Allow",
		"Principal": {
			"Service": "spotfleet.amazonaws.com"
		},
		"Action": "sts:AssumeRole"
	}
]
}
EOF
}

resource "aws_iam_role_policy" "rattlesnake_spot_fleet_policy" {
	name = "${var.name}-spot-fleet-policy"
	role = "${aws_iam_role.rattlesnake_spot_fleet_role.id}"
	policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
	{
		"Effect": "Allow",
		"Action": "*",
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
  acl    = "private"

  lifecycle_rule {
    id      = "incremental"
    enabled = true
    prefix  = "${var.device}-incremental"

    expiration {
      days = 30
    }
	}

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
	timeout          = "20"
}

###################
# Cloudwatch Event
###################
resource "aws_cloudwatch_event_rule" "every_day" {
    name = "${var.name}-daily-check"
    description = "RattlesnakeOS build"
    schedule_expression = "<% .Schedule %>"
}

resource "aws_cloudwatch_event_target" "check_build_every_day" {
    rule = "${aws_cloudwatch_event_rule.every_day.name}"
    target_id = "${var.name}-build"
    arn = "${aws_lambda_function.rattlesnake_lambda_build.arn}"
}

resource "aws_lambda_permission" "allow_cloudwatch_to_call_check_foo" {
    statement_id = "AllowExecutionFromCloudWatch"
    action = "lambda:InvokeFunction"
    function_name = "${aws_lambda_function.rattlesnake_lambda_build.function_name}"
    principal = "events.amazonaws.com"
    source_arn = "${aws_cloudwatch_event_rule.every_day.arn}"
}
 
###################
# Outputs
###################
output "sns_topic_arn" {
	description = "The SNS ARN"
	value = "${aws_sns_topic.rattlesnake.arn}"
}
output "iam_fleet_role_arn" {
	description = "The Fleet Role ARN"
	value = "${aws_iam_role.rattlesnake_spot_fleet_role.arn}"
}
output "iam_ec2_instance_profile_arn" {
	description = "The EC2 instance profile ARN"
	value = "${aws_iam_instance_profile.rattlesnake_ec2_role.arn}"
}
`
