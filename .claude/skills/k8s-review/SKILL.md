# K8s-Review: Kubernetes Architecture Review for Docker Desktop

## Purpose

Review Kubernetes manifests and Helm charts for production-readiness concerns that `helm lint` and `kubeconform` miss, with specific focus on running on Docker Desktop Kubernetes on Mac M4 (ARM64).

## When to Use

Use this skill when:
- Adding or modifying Helm charts in `deploy/helm/`
- Changing resource requests/limits, HPA, or PDB configuration
- Before deploying to the local Docker Desktop Kubernetes cluster
- Reviewing readiness/liveness probes and graceful shutdown configuration

## Prerequisite

**`make k8s-lint` must pass before this skill runs.**

Run `make k8s-lint` first. If it fails, fix all linting and schema validation errors before proceeding. This skill focuses on operational concerns that schema validators cannot catch.

## Process

### Step 1: Verify Prerequisite

Run `make k8s-lint` and confirm it passes with zero errors. If it fails, stop and report the failures to the user. Do not proceed with architecture review until k8s linting is clean.

### Step 2: Identify Scope

Determine which charts and manifests to review:
- Use `git diff --name-only` to find changed files under `deploy/`
- If no diff context, review all files in `deploy/helm/`
- Also check any Kubernetes-related configuration in `docker-compose*.yml`

### Step 3: Review Resource Sizing for M4 Docker Desktop

Docker Desktop on Mac M4 has constrained resources shared with the host OS. Review for:

- **Over-provisioned requests:** CPU/memory requests that are too high for a local dev cluster. Docker Desktop typically has 4-8 CPU cores and 8-16 GB RAM allocated. A single service should not request more than 500m CPU / 512Mi memory unless justified.
- **Missing resource limits:** Every container must have both requests and limits set. Missing limits risk OOM-killing other workloads in the shared cluster.
- **Request/limit ratio:** Limits should not exceed 2x requests for CPU or 1.5x for memory. Large ratios cause unpredictable scheduling and throttling.
- **Init container resources:** Init containers (migrations, config loaders) often have copy-pasted resource specs from the main container. They typically need far less.
- **Sidecar overhead:** If using sidecars (envoy, log collectors), account for their resource consumption in the total pod budget.

### Step 4: Review HPA Configuration

Check Horizontal Pod Autoscaler settings for local dev sanity:

- **minReplicas too high:** On Docker Desktop, `minReplicas` should typically be 1 for dev workloads. Setting it higher wastes resources on a constrained cluster.
- **maxReplicas too high:** On Docker Desktop, `maxReplicas` above 3-4 will likely exhaust cluster resources. Flag anything above 5.
- **Scaling metrics:** Verify the HPA targets metrics that are actually available. Custom metrics require metrics-server or a metrics adapter. Docker Desktop ships with metrics-server but custom metrics need extra setup.
- **Scale-down stabilization:** Default `scaleDown.stabilizationWindowSeconds` is 300s (5 min). For dev, shorter windows (60-120s) give faster feedback.
- **CPU vs Memory targeting:** HPA on memory is risky for Go services (GC can cause flapping). Prefer CPU-based or custom request-rate metrics.

### Step 5: Review Graceful Shutdown

Verify the service shuts down cleanly when Kubernetes sends SIGTERM:

- **preStop hook:** Check for a `preStop` lifecycle hook. Services behind a load balancer need a small sleep (3-5s) to allow endpoint propagation before starting shutdown.
- **terminationGracePeriodSeconds:** Default is 30s. Verify it's long enough for in-flight requests to drain. For services with long-running operations (Kafka consumers, batch processing), this may need to be longer.
- **SIGTERM handling in code:** Cross-reference with Go code — the service should catch SIGTERM and drain HTTP connections (`http.Server.Shutdown`), close database pools, and commit/flush Kafka consumer offsets.
- **Readiness probe alignment:** The readiness probe should start failing as soon as shutdown begins, so the service is removed from the endpoint list before connections are refused.

### Step 6: Review PDB Configuration

Check PodDisruptionBudget settings:

- **PDB exists for stateful workloads:** Any service that handles state (leader election, in-memory cache, connection pools) should have a PDB.
- **minAvailable vs maxUnavailable:** For a dev cluster with 1-2 replicas, `minAvailable: 1` with 2 replicas means rolling updates will work. `minAvailable: 1` with 1 replica will block voluntary disruptions entirely (node drain, cluster upgrade).
- **PDB blocking node drain:** On Docker Desktop, Kubernetes upgrades drain the single node. If PDB is too strict with low replica counts, it will block upgrades. Flag `minAvailable` >= `replicas` as a problem.
- **Missing PDB:** If the service runs with `replicas > 1`, it should have a PDB to prevent all pods being evicted simultaneously during voluntary disruptions.

### Step 7: Review Probes

Check liveness and readiness probe configuration:

- **Liveness probe too aggressive:** `failureThreshold * periodSeconds` should be at least 30s. Aggressive liveness probes cause unnecessary restarts during GC pauses or transient load spikes.
- **Readiness probe dependencies:** Readiness probes should check downstream dependencies (database, Redis) that the service needs to serve traffic. A service that passes readiness but can't reach its database will receive traffic it can't handle.
- **Startup probe for slow starts:** If the service takes more than 10s to start (migrations, cache warming), use a `startupProbe` instead of increasing `initialDelaySeconds` on the liveness probe.
- **Probe endpoints:** Verify the probe endpoints exist in the Go code and return appropriate status codes.

### Step 8: Review Additional Concerns

- **Image pull policy:** Should be `IfNotPresent` for local builds, `Always` for remote images. Wrong policy causes unnecessary pulls or stale images.
- **Node affinity / tolerations:** Docker Desktop is a single-node cluster. Node affinity rules and tolerations are ignored but add confusion. Flag unnecessary scheduling constraints.
- **PVC sizing:** Persistent volumes on Docker Desktop use the host filesystem. Over-sized PVCs waste disk space. Verify PVC sizes are reasonable for dev data volumes.
- **Service type:** `LoadBalancer` on Docker Desktop maps to localhost ports. Verify no port conflicts. `NodePort` is an alternative that avoids the Docker Desktop load balancer behavior.
- **ARM64 image compatibility:** All images must support `linux/arm64`. Flag any images that only publish `amd64` manifests.

## Output Format

Present findings as a markdown table:

```markdown
## Kubernetes Architecture Review Results

**Prerequisite:** `make k8s-lint` passed
**Target environment:** Docker Desktop Kubernetes on Mac M4 (ARM64)

| File | Concern | Severity | Recommendation |
|------|---------|----------|----------------|
| `deploy/helm/templates/deployment.yaml` | CPU request 2000m too high for Docker Desktop | HIGH | Reduce to 250m-500m for local dev |
| `deploy/helm/values.yaml` | HPA maxReplicas=10, will exhaust cluster resources | HIGH | Set maxReplicas to 3-4 for Docker Desktop |
| `deploy/helm/templates/deployment.yaml` | No preStop hook, traffic may hit during shutdown | HIGH | Add `preStop` with 5s sleep for endpoint propagation |
| `deploy/helm/templates/pdb.yaml` | minAvailable=2 with replicas=2 blocks node drain | MEDIUM | Use maxUnavailable=1 or reduce minAvailable to 1 |
| `deploy/helm/values.yaml` | Liveness probe failureThreshold=1, periodSeconds=5 | MEDIUM | Increase to failureThreshold=3 to tolerate transient issues |
| `deploy/helm/templates/deployment.yaml` | Init container has same resource requests as main container | LOW | Reduce init container to 100m CPU / 128Mi memory |
```

### Severity Levels

- **HIGH:** Will cause outages, resource exhaustion, or broken deployments on Docker Desktop
- **MEDIUM:** Suboptimal configuration that causes flapping, slow rollouts, or wasted resources
- **LOW:** Convention improvements, unnecessary configuration for a single-node dev cluster

## Notes

- This review complements `make k8s-lint` — never duplicate what `helm lint` or `kubeconform` already catches
- Always consider the Docker Desktop single-node constraint when evaluating findings
- Cross-reference probe endpoints and shutdown behavior with the Go application code
- Resource recommendations are for local development; production values will differ
- If the chart supports multiple environments via values files, review both dev and prod values
