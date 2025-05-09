# armada-scheduler-migration

![Version: 0.0.0-latest](https://img.shields.io/badge/Version-0.0.0--latest-informational?style=flat-square) ![AppVersion: 0.0.0-latest](https://img.shields.io/badge/AppVersion-0.0.0--latest-informational?style=flat-square)

A Helm chart for the Armada Scheduler database migration

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalLabels | object | `{}` |  |
| additionalVolumeMounts | list | `[]` |  |
| additionalVolumes | list | `[]` |  |
| applicationConfig | object | `{}` |  |
| args.timeout | string | `"5m"` |  |
| customServiceAccount | string | `nil` |  |
| image.repository | string | `"gresearchdev/armada-scheduler"` |  |
| image.tag | string | `"0.0.0-latest"` |  |
| resources.limits.cpu | string | `"200m"` |  |
| resources.limits.memory | string | `"256Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| serviceAccount | string | `nil` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
