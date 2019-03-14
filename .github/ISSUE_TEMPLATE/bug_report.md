---
name: Bug report
about: Create a report to help us improve

---

### Prerequisites

* [ ] I am running the [latest version](https://github.com/dan-v/rattlesnakeos-stack/releases) of rattlesnakeos-stack
* [ ] I am able to reproduce this issue without any advanced customization options

### Description

[Description of the issue]

### Steps to Reproduce

1. [First Step]
2. [Second Step]
3. [and so on...]

**Expected behavior:** [What you expected to happen]

**Actual behavior:** [What actually happened]

### Platform
e.g. OSX, Linux, Windows

### Full Config Used for Setup
You can mask stack name and email from here.
```
chromium-version = ""
device = "crosshatch"
encrypted-keys = true
ignore-version-checks = false
instance-regions = "us-west-2,us-west-1,us-east-1,us-east-2"
instance-type = "c5.4xlarge"
max-price = "1.00"
name = "<not important>"
region = "us-west-2"
schedule = "cron(0 0 10 * ? *)"
skip-price = "1.00"
ssh-key = "rattlesnakeos"
```

### Carrier
e.g. T-mobile, etc

### Email Notification Output
Should look something like this. You can mask stack name from here and you may want to verify contents of log output before pasting.
```
RattlesnakeOS build FAILED
 Device: crosshatch
 Stack Name: <not important>
 Stack Version: 9.0.24 
 Stack Region: us-west-2
 Release Channel: crosshatch-stable
 Instance Type: c5.4xlarge
 Instance Region: us-east-2
 Instance IP: 3.17.133.131
 Build Date: 2019.03.03.06
 Elapsed Time: 2hrs 3min 1sec
 AOSP Build: PQ2A.190205.001
 AOSP Branch: android-9.0.0_r31
 Chromium Version: 72.0.3626.121
 F-Droid Version: 1.5.1
 F-Droid Priv Extension Version: 0.2.9
...
... a bunch of log output...
...
```

### Full log
Sometimes the output from the email notification will not be enough to diagnose the issue. Including the full log file can help. You can find log files in your S3 bucket `<stackname>-logs/<device>/<timestamp>`. If you don't want to share it publicly we can find a way to share and debug it offline.
