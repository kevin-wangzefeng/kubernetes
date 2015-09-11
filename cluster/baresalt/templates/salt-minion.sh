#!/bin/bash

# Copyright 2014 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# exit on any error
set -e

#setup kubelet config
mkdir -p "/var/lib/kubelet"
(umask 077;
cat > "/var/lib/kubelet/kubeconfig" << EOF
apiVersion: v1
kind: Config
users:
- name: kubelet
user:
  token: ${KUBELET_TOKEN}
clusters:
- name: local
cluster:
  insecure-skip-tls-verify: true
contexts:
- context:
  cluster: local
  user: kubelet
name: service-account-context
current-context: service-account-context
EOF
)

#setup proxy config
mkdir -p "/var/lib/kube-proxy/"
# Make a kubeconfig file with the token.
# TODO(etune): put apiserver certs into secret too, and reference from authfile,
# so that "Insecure" is not needed.
(umask 077;
cat > "/var/lib/kube-proxy/kubeconfig" << EOF
apiVersion: v1
kind: Config
users:
- name: kube-proxy
user:
  token: ${KUBE_PROXY_TOKEN}
clusters:
- name: local
cluster:
   insecure-skip-tls-verify: true
contexts:
- context:
  cluster: local
  user: kube-proxy
name: service-account-context
current-context: service-account-context
EOF
)

# Setup hosts file to support ping by hostname to master
if [ ! "$(cat /etc/hosts | grep $MASTER_NAME)" ]; then
  echo "Adding $MASTER_NAME to hosts file"
  echo "$MASTER_IP $MASTER_NAME" >> /etc/hosts
fi

# Setup hosts file to support ping by hostname to each minion in the cluster
for (( i=0; i<${#MINION_NAMES[@]}; i++)); do
  minion=${MINION_NAMES[$i]}
  ip=${NODES[$i]}
  if [ ! "$(cat /etc/hosts | grep $minion)" ]; then
    echo "Adding $minion to hosts file"
    echo "$ip $minion" >> /etc/hosts
  fi
done

# Placeholder for any other manifests that may be per-node.
mkdir -p /etc/kubernetes/manifests

# Prepopulate the name of the Master
mkdir -p /etc/salt/minion.d
echo "master: $MASTER_NAME" > /etc/salt/minion.d/master.conf

cat <<EOF >/etc/salt/minion.d/log-level-debug.conf
log_level: debug
log_level_logfile: debug
EOF

hostnamef=$(uname -n)
#apt-get install -y ipcalc
#netmask=$(ipcalc $MINION_IP_RANGE | grep Netmask | awk '{ print $2 }')
#network=$(ipcalc $MINION_IP_RANGE | grep Address | awk '{ print $2 }')
#cbrstring="$network $netmask"

# Our minions will have a pool role to distinguish them from the master.
cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-pool
  hostnamef: $hostnamef
EOF
#TODO: cloud: vagrant

install-salt

# Wait a few minutes and trigger another Salt run to better recover from
# any transient errors.
echo "Sleeping 10"
sleep 10
salt-call state.highstate || true
