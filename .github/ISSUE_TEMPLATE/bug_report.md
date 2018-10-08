---
name: Bug report
about: Create a report to help us improve

---

### Prerequisites

* [ ] I am running the [latest version](https://github.com/dan-v/rattlesnakeos-stack/releases) of rattlesnakeos-stack
* [ ] I am able to reproduce this issue without any advanced customization options like --repo-patches or --repo-apks flags

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

### Full Command Used for Setup
e.g. ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-dan --device marlin

### Carrier
e.g. T-mobile, etc

### Email Notification output
Should look something like this:
```
RattlesnakeOS build FAILED
  Device: marlin
  Stack Name: rattlesnakeos-stackname
  Stack Version: 9.0.7
  Stack Region: us-west-2
  Release Channel: marlin-stable
  Instance Type: c5.4xlarge
  Instance Region: us-east-2
  Instance IP: 18.18.0.1
  Build Date: 2018.10.07.23
  Elapsed Time: 0hrs 0min 8sec
  AOSP Build: PPR2.181005.003
  AOSP Branch: android-9.0.0_r10
  Chromium Version: 69.0.3497.100
  F-Droid Version: 1.4
  F-Droid Priv Extension Version: 0.2.8
...
... a bunch of log output...
...
```

### Full log
Sometimes the output from the email notification will not be enough to diagnose the issue. Including the full log file can help. You can find log files in your S3 bucket `<stackname>-logs/<device>/<timestamp>`
