# KubeMeter

KubeMeter 是一个通用的 Kubernetes 资源计量系统，用于计量用户申请的计算资源（CPU、内存、GPU）和存储资源（PVC），支持多级父资源信息，便于业务端按照父资源维度进行计费。

## 特性

- **基于申请量计费**：计量 Pod 的 `requests` 和 PVC 的 `spec.resources.requests`，而非实际使用量
- **多资源类型支持**：CPU、内存、GPU（nvidia.com/gpu）、存储（PVC）
- **灵活的资源选择器**：支持通过命名空间、标签选择器筛选资源
- **多级父资源信息**：自动从资源标签中提取父资源信息，支持业务侧按层级计费
- **自动化计量**：定期收集资源使用数据，自动计算运行时长
- **声明式配置**：通过 CRD 定义计量规则

## 架构

```
ResourceMeter (计量配置)
    ↓ 定义选择器和计量维度
    ↓
ResourceMeter Controller
    ↓ 监控并筛选资源
    ├─ Pods (开发机、训练任务等)
    └─ PersistentVolumeClaims (存储)
    ↓ 读取 requests 并计算时长
ResourceUsage (计量结果)
    ↓ 包含资源分配量和运行时长
业务端计费系统
```

## 快速开始

### 前置条件

- Kubernetes 集群 1.20+
- kubectl 配置正确

### 安装

```bash
# 克隆仓库
git clone https://github.com/wutz/kubemeter.git
cd kubemeter

# 安装 CRDs
make install

# 部署 operator
make deploy IMG=ghcr.io/wutz/kubemeter:latest
```

### 创建计量配置

#### 计量 Pod 资源

```yaml
apiVersion: metering.paratera.com/v1alpha1
kind: ResourceMeter
metadata:
  name: pod-meter
  namespace: default
spec:
  selector:
    namespaces:
      - default
    labelSelector:
      matchLabels:
        app: myapp
  resources:
    - type: cpu
      unit: core-hour
    - type: memory
      unit: GiB-hour
    - type: gpu
      unit: gpu-hour
  collectionInterval: 1m
  retentionPeriod: 168h  # 7 days
```

#### 计量 PVC 资源

```yaml
apiVersion: metering.paratera.com/v1alpha1
kind: ResourceMeter
metadata:
  name: pvc-meter
  namespace: default
spec:
  selector:
    namespaces:
      - default
  resources:
    - type: storage
      unit: GiB
  collectionInterval: 5m
```

### 查看计量结果

```bash
# 查看所有计量配置
kubectl get resourcemeters

# 查看计量结果
kubectl get resourceusages

# 查看详细信息
kubectl describe resourceusage pod-meter-<pod-name>
```

## API 参考

### ResourceMeter

| 字段 | 类型 | 描述 |
|------|------|------|
| `spec.selector` | ResourceSelector | 资源选择器 |
| `spec.resources[]` | ResourceMetric | 要计量的资源类型列表 |
| `spec.collectionInterval` | Duration | 数据收集间隔（默认 1m） |
| `spec.retentionPeriod` | Duration | 数据保留时长（默认 168h） |

### ResourceUsage

| 字段 | 类型 | 描述 |
|------|------|------|
| `spec.meterRef` | string | 关联的 ResourceMeter 名称 |
| `spec.resourceRef` | ResourceReference | 被计量的资源引用 |
| `spec.parentRefs[]` | ResourceReference | 父资源信息列表 |
| `spec.timeRange` | TimeRange | 时间范围 |
| `status.allocations` | ResourceAllocations | 资源申请量 |
| `status.runningHours` | string | 运行时长（小时） |
| `status.cost` | string | 计费金额（可选） |

### 资源类型

- `cpu`: CPU 核心数
- `memory`: 内存字节数
- `gpu`: GPU 数量（nvidia.com/gpu）
- `storage`: 存储字节数

## 多级父资源信息

KubeMeter 自动从资源标签中提取父资源信息，支持常见的标签模式：

- `app.kubernetes.io/name` + `app.kubernetes.io/component`
- `app` + `component`
- `owner` + `owner-kind`

示例：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: training-job-worker-1
  labels:
    app.kubernetes.io/name: training-platform
    app.kubernetes.io/component: worker
    project: nlp-research
    team: ai-lab
```

生成的 ResourceUsage 将包含：

```yaml
spec:
  parentRefs:
    - name: training-platform
      kind: worker
      labels:
        project: nlp-research
        team: ai-lab
```

业务端可以根据 `parentRefs` 和 `labels` 进行多维度计费聚合。

## 与 Prometheus 的关系

KubeMeter 专注于**计费**，基于资源申请量（requests）计算费用。

Prometheus 专注于**监控**，提供实际使用率分析。

两者互补：
- KubeMeter 提供准确、公平的计费数据
- Prometheus 帮助用户发现资源浪费，优化申请量

## 开发

### 本地运行

```bash
# 安装 CRDs
make install

# 运行 controller（使用当前 kubeconfig）
make run
```

### 测试

```bash
# 运行单元测试
make test

# 运行 lint
make lint-fix
```

### 构建镜像

```bash
export IMG=<your-registry>/kubemeter:<tag>
make docker-build docker-push IMG=$IMG
```

## 部署选项

### 使用 Kustomize

```bash
make build-installer IMG=<your-registry>/kubemeter:<tag>
kubectl apply -f dist/install.yaml
```

### 使用 Helm

```bash
# 生成 Helm chart
kubebuilder edit --plugins=helm/v2-alpha

# 部署
helm install kubemeter ./dist/chart/ --namespace kubemeter-system --create-namespace
```

## 贡献

欢迎贡献！请提交 Issue 或 Pull Request。

## 许可证

Apache License 2.0

## 联系方式

- 作者：Taizeng Wu
- 仓库：https://github.com/wutz/kubemeter
