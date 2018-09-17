---
name: Bug report
about: Create a report to help us improve

---

### Prerequisites

* [ ] Are you running the [latest version](https://github.com/dan-v/rattlesnakeos-stack/releases)?

### Description

[Description of the bug]

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
  Stack Version: 0.0.7
  Build Date: 2018.06.23.06
  Elapsed Time: 1hrs 0min 8sec
  AOSP Build: OPM4.171019.021.D1
  AOSP Branch: android-8.1.0_r31
  Kernel Branch: android-msm-marlin-3.18-oreo-m4
  Chromium Version: 67.0.3396.87
  F-Droid Version: 1.2.2
  F-Droid Priv Extension Version: 0.2.8
```

### Log output
You can find log files in your S3 bucket `<stackname>-logs/<device>/<timestamp>`
