/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docker

import (
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-docker/docker/actions"
	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// Node is a node initializer. It knows how to build every kind of node
type Node struct {
	Cluster, Machine, Role, Version string
	Nodes                           []nodes.Node
	MachineActions                  *actions.MachineActions
	logr.Logger
}

// NewNode creates a node initializer
func NewNode(cluster, machine, role, version string, log logr.Logger) (*Node, error) {
	clusterNodes, err := nodes.List("label=" + constants.ClusterLabelKey + "=" + cluster)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list nodes")
	}
	return &Node{
		Logger:         log.WithName("new-node").WithValues("cluster", cluster, "machine", machine, "role", role, "version", version),
		Cluster:        cluster,
		Machine:        machine,
		Role:           role,
		Version:        version,
		Nodes:          clusterNodes,
		MachineActions: &actions.MachineActions{Logger: log.WithName("machine-actions")},
	}, nil
}

// Create figures out what kind of node to make and does the right thing
func (n *Node) Create(cloudConfig []byte) (*nodes.Node, error) {
	log := n.Logger.WithName("node-create")
	switch n.Role {
	case constants.ControlPlaneNodeRoleValue:
		// Node length includes ELB which must already exist
		if len(n.Nodes) == 1 {
			log.Info("Creating the first control plane node")
			cp, err := n.MachineActions.CreateControlPlane(n.Cluster, n.Machine, n.Version, nil, cloudConfig)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create control plane")
			}

			// save the kubeconfig on the host with the loadbalancer endpoint
			dest := actions.KubeConfigPath(n.Cluster)
			hostAddress := "127.0.0.1"
			_, hostPort, err := actions.GetLoadBalancerHostAndPort(n.Nodes)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get load balancer port")
			}

			if err := actions.GetKubeConfig(cp, dest, hostAddress, hostPort); err != nil {
				return nil, errors.Wrap(err, "failed to get kubeconfig from node")
			}
		}
		log.Info("Adding an additional control plane node")
		return n.MachineActions.AddControlPlane(n.Cluster, n.Machine, n.Version, cloudConfig)
	case constants.WorkerNodeRoleValue:
		log.Info("Adding a worker")
		return n.MachineActions.AddWorker(n.Cluster, n.Machine, n.Version, cloudConfig)
	default:
		log.Info("Unknown role", "role", n.Role)
		return nil, errors.Errorf("Unknown role: %q", n.Role)
	}
}

// Delete removes the underlying infrastructure for a given Node.
func (n *Node) Delete() error {
	log := n.Logger.WithName("node-delete")
	switch n.Role {
	case constants.ControlPlaneNodeRoleValue:
		return n.MachineActions.DeleteControlPlane(n.Cluster, n.Machine)
	case constants.WorkerNodeRoleValue:
		return n.MachineActions.DeleteWorker(n.Cluster, n.Machine)
	default:
		log.Info("Unknown role", "role", n.Role)
		return errors.Errorf("Unknown role: %q", n.Role)
	}
}