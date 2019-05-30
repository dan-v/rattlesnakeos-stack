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

variable "enable_attestation" {
    description = "Whether to enable attestation server"
    default     = "<% .Config.EnableAttestation %>"
}

variable "attestation_instance_type" {
    description = "Instance type for attestation server"
    default     = "<% .Config.AttestationInstanceType %>"
}

variable "attestation_max_spot_price" {
    description = "Max spot price for attestation server"
    default     = "<% .Config.AttestationMaxSpotPrice %>"
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
    }<% if .Config.EnableAttestation %>,
    {
        "Action": [
            "ec2:*",
            "autoscaling:*",
            "elasticbeanstalk:*"
        ],
        "Effect": "Allow",
        "Resource": "*"
    },
    {
        "Action": [
          "sns:CreateTopic",
          "sns:GetTopicAttributes",
          "sns:ListSubscriptionsByTopic",
          "sns:Subscribe"
        ],
        "Effect": "Allow",
        "Resource": "*"
    },
    {
        "Action": [
            "s3:*"
        ],
        "Effect": "Allow",
        "Resource": [
            "arn:aws:s3:::elasticbeanstalk-*",
            "arn:aws:s3:::elasticbeanstalk-*/*"
        ]
    },
    {
        "Effect": "Allow",
        "Action": [
            "cloudformation:*"
        ],
        "Resource": [
            "arn:aws:cloudformation:*:*:stack/awseb-*",
            "arn:aws:cloudformation:*:*:stack/eb-*"
        ]
    },
    {
        "Effect": "Allow",
        "Action": [
            "iam:PassRole"
        ],
        "Resource": "arn:aws:iam::*:role/${var.name}-beanstalk-ec2"
    }
    <% end %>
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

resource "aws_iam_policy" "rattlesnake_spot_fleet_policy" {
    name = "${var.name}-spot-fleet-policy"
    policy = <<EOF
{
"Version": "2012-10-17",
"Statement": [
    {
        "Effect": "Allow",
        "Action": [
                "ec2:DescribeImages",
                "ec2:DescribeSubnets",
                "ec2:RequestSpotInstances",
                "iam:CreateServiceLinkedRole",
                "ec2:TerminateInstances",
                "ec2:DescribeInstanceStatus",
                "iam:PassRole"
        ],
        "Resource": [
                "*"
        ]
},
{
        "Effect": "Allow",
        "Action": [
                "elasticloadbalancing:RegisterInstancesWithLoadBalancer"
        ],
        "Resource": [
                "arn:aws:elasticloadbalancing:*:*:loadbalancer/*"
        ]
},
{
        "Effect": "Allow",
        "Action": [
                "elasticloadbalancing:RegisterTargets"
        ],
        "Resource": [
                "*"
        ]
}
]
}
EOF
}

resource "aws_iam_role_policy_attachment" "rattlesnake_spot_fleet_policy_attachment" {
    role       = "${aws_iam_role.rattlesnake_spot_fleet_role.name}"
    policy_arn = "${aws_iam_policy.rattlesnake_spot_fleet_policy.arn}"
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

<% if .Config.EnableAttestation %>
###################
# Attestation
###################
data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {}

resource "aws_vpc" "vpc" {
    cidr_block = "10.0.0.0/16"
    enable_dns_support = "true"
    enable_dns_hostnames = "true"
    tags {
        Name = "${var.name}"
    }
}

resource "aws_subnet" "subnet_public" {
    count = "${length(data.aws_availability_zones.available.names)}"
    vpc_id            = "${aws_vpc.vpc.id}"
    cidr_block        = "${cidrsubnet("10.0.0.0/16", 8, count.index)}"
    availability_zone = "${data.aws_availability_zones.available.names[count.index]}"
    tags {
        Name          = "${var.name}-public-${data.aws_availability_zones.available.names[count.index]}"
    }
}

resource "aws_internet_gateway" "vpc-igw" {
    vpc_id = "${aws_vpc.vpc.id}"
}

resource "aws_route_table" "route_to_igw" {
    vpc_id = "${aws_vpc.vpc.id}"
    route {
        cidr_block = "0.0.0.0/0"
        gateway_id = "${aws_internet_gateway.vpc-igw.id}"
    }
    tags {
        Name = "${var.name}-route-to-internet-via-igw"
    }
}

resource "aws_route_table_association" "pub_to_igw_association" {
    subnet_id      = "${aws_subnet.subnet_public.*.id[count.index]}"
    route_table_id = "${aws_route_table.route_to_igw.id}"
    count = "${length(data.aws_availability_zones.available.names)}"
}

resource "aws_security_group" "beanstalk" {
    vpc_id = "${aws_vpc.vpc.id}"
    name = "${var.name}-beanstalk-attestation-sg"
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }

    ingress {
        from_port = 22
        to_port = 22
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    ingress {
        from_port = 80
        to_port = 80
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    ingress {
        from_port = 443
        to_port = 443
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    tags {
      Name = "${var.name}"
    }
}

resource "aws_s3_bucket" "rattlesnake_s3_attestation" {
    bucket = "${var.name}-attestation"
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

data "aws_elastic_beanstalk_solution_stack" "docker" {
    name_regex          = "^64bit Amazon Linux (.*) running Docker(.*)$"
    most_recent         = true
}

resource "aws_iam_role" "rattlesnake_beanstalk_ec2_role" {
    name = "${var.name}-beanstalk-ec2"
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

resource "aws_iam_role_policy" "rattlesnake_beanstalk_ec2_policy" {
    name = "${var.name}-beanstalk-ec2-policy"
    role = "${aws_iam_instance_profile.rattlesnake_beanstalk_ec2_role.id}"
    policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "BucketAccess",
            "Action": [
                "s3:Get*",
                "s3:List*",
                "s3:PutObject"
            ],
            "Effect": "Allow",
            "Resource": [
                "arn:aws:s3:::elasticbeanstalk-*",
                "arn:aws:s3:::elasticbeanstalk-*/*",
                "arn:aws:s3:::${var.name}-attestation*",
                "arn:aws:s3:::${var.name}-attestation*/*"
            ]
        },
        {
            "Sid": "SNSAccess",
            "Effect": "Allow",
            "Action": [
                "sns:ListTopics",
                "sns:Publish"
            ],
            "Resource": "*"
        },
        {
            "Sid": "XRayAccess",
            "Action": [
                "xray:PutTraceSegments",
                "xray:PutTelemetryRecords",
                "xray:GetSamplingRules",
                "xray:GetSamplingTargets",
                "xray:GetSamplingStatisticSummaries"
            ],
            "Effect": "Allow",
            "Resource": "*"
        },
        {
            "Sid": "CloudWatchLogsAccess",
            "Action": [
                "logs:PutLogEvents",
                "logs:CreateLogStream",
                "logs:DescribeLogStreams",
                "logs:DescribeLogGroups"
            ],
            "Effect": "Allow",
            "Resource": [
                "arn:aws:logs:*:*:log-group:/aws/elasticbeanstalk*"
            ]
        }
    ]
}
EOF
}

resource "aws_iam_instance_profile" "rattlesnake_beanstalk_ec2_role" {
    name = "${var.name}-beanstalk-ec2"
    role = "${aws_iam_role.rattlesnake_beanstalk_ec2_role.name}"
}

resource "aws_elastic_beanstalk_application" "attestation" {
    name        = "${var.name}-attestation"
    description = "attestation"

    appversion_lifecycle {
        service_role          = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/aws-elasticbeanstalk-service-role"
        max_count             = 2
        delete_source_from_s3 = true
    }
}

resource "aws_elastic_beanstalk_environment" "attestation" {
    name                = "attestation"
    application         = "${aws_elastic_beanstalk_application.attestation.name}"
    solution_stack_name = "${data.aws_elastic_beanstalk_solution_stack.docker.name}"

    setting {
        namespace = "aws:elasticbeanstalk:environment"
        name      = "EnvironmentType"
        value     = "SingleInstance"
    }

    setting {
        namespace = "aws:autoscaling:launchconfiguration"
        name      = "InstanceType"
        value     = "${var.attestation_instance_type}"
    }

    setting {
        namespace = "aws:autoscaling:asg"
        name      = "MaxSize"
        value     = "1"
    }

    setting {
        namespace = "aws:ec2:vpc"
        name = "VPCId"
        value = "${aws_vpc.vpc.id}"
    }

    setting {
        namespace = "aws:ec2:vpc"
        name = "Subnets"
        value = "${join(",", aws_subnet.subnet_public.*.id)}"
    }

    setting {
        namespace = "aws:autoscaling:launchconfiguration"
        name      = "SecurityGroups"
        value     = "${aws_security_group.beanstalk.id}"
    }

    setting {
        namespace  = "aws:autoscaling:launchconfiguration"
        name       = "IamInstanceProfile"
        value      = "${aws_iam_role.rattlesnake_beanstalk_ec2_role.name}"
    }

    setting {
        namespace = "aws:elasticbeanstalk:sns:topics"
        name      = "Notification Topic ARN"
        value     = "${aws_sns_topic.rattlesnake.arn}"
    }

    setting {
        namespace = "aws:autoscaling:launchconfiguration"
        name      = "BlockDeviceMappings"
        value     = "/dev/xvdcz=:5:true:gp2"
    }
}
<% end %>

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
`
