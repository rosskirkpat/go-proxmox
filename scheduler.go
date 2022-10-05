package proxmox

import (
	"fmt"
	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/kubernetes/pkg/apis/core"
	"net/http"
)

const ProviderID = "proxmox"

type ResourceToAnnotationMaps map[string]string

func schedule() {
	router := mux.NewRouter()
	router.Get("/")
	router.Methods("POST", "GET")
}

type Translator struct{}

type ProxmoxCluster struct {
	Annotations map[string]string
	Labels      map[string]string

	Nodes      []corev1.Node
	Pods       []corev1.Pod
	Namespaces []corev1.Namespace
	Containers []corev1.Container

	metav1.TypeMeta
	metav1.ObjectMeta
}

type KCluster struct {
	Name string
	Id   string
	http.Client
	Node    corev1.Node
	Cluster ProxmoxCluster
}

// translate proxmox objects/resources to k8s/scheduler resources/objects

// Session

// Credentials

// VNC

// Cluster translator
func (t *Translator) Cluster(c Cluster) {
	kc := KCluster{}
	c.client.httpClient = &kc.Client
	c.ID = kc.Id
	c.Name = kc.Name
}

// ClusterResource translator
func (t *Translator) ClusterResource() {}

// NodeStatus translator
func (t *Translator) NodeStatus(c Cluster) {}

// Node translator
func (t *Translator) Node() {}

// Task translator
func (t *Translator) Task() {}

// VirtualMachine translator
func (t *Translator) VirtualMachine(vm VirtualMachine) {
	kc := KCluster{}
	n := corev1.Node{}
	vm.Node = kc.Name // proxmox node running the VM maps to a cluster name, abstraction goal is 1 proxmox node = 1 kubernetes cluster
	vm.Name = n.Name  // proxmox vm maps to a node name, abstraction goal is 1 proxmox node = 1 kubernetes node
	vm.client.httpClient = &kc.Client
	vm.Status = string(n.Status.Phase)

	n.Spec.ProviderID = ProviderID
	kc.Cluster.Annotations = ResourceToAnnotationMaps{
		"cpu.node.proxmox.io":    fmt.Sprintf("%v", vm.CPU),
		"memory.node.proxmox.io": fmt.Sprintf("%v", vm.Mem),
		"disk.node.proxmox.io":   fmt.Sprintf("%v", vm.Disk),
		"uptime.node.proxmox.io": fmt.Sprintf("%v", vm.Uptime),
		"cpus.node.proxmox.io":   fmt.Sprintf("%v", vm.CPUs),
	}

}

// CPU translator
func (t *Translator) CPU() {}

// Memory translator

func (t *Translator) Memory() {}

// VirtualMachineConfig translator

func (t *Translator) VirtualMachineConfig() {}

// Time translator
func (t *Translator) Time() {}

// Container translator
func (t *Translator) Container() {}

// Appliance translator
func (t *Translator) Appliance() {}

// Storage translator
func (t *Translator) Storage() {}

// NodeNetwork Translator
func (t *Translator) NodeNetwork() {}
