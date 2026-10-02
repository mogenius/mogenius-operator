package dtos

// PvcFileRequestDto addresses a path for the files/v2 patterns. Two ways to
// name where the path lives, exactly one of them set:
//
//   - PvcName: a volume; the operator picks a running pod that mounts it (or
//     its storage helper) and resolves the path against the mount.
//   - Pod: a running pod's own filesystem — what the sandbox toolbox uses. The
//     path is absolute inside the container; Container selects one of the
//     pod's containers and defaults to the first. An image without a shell
//     is served through the ephemeral debug container, where the target's
//     root is /proc/1/root.
type PvcFileRequestDto struct {
	Namespace string `json:"namespace" validate:"required"`
	PvcName   string `json:"pvcName,omitempty" validate:"required_without=Pod"`
	Pod       string `json:"pod,omitempty" validate:"required_without=PvcName"`
	Container string `json:"container,omitempty"`
	Path      string `json:"path" validate:"required"`
}

// AddressesPod reports whether the request names a pod rather than a volume.
func (r PvcFileRequestDto) AddressesPod() bool {
	return r.Pod != ""
}
