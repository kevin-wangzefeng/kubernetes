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

# A library of helper functions and constant for the local config.

# Use the config file specified in $KUBE_CONFIG_FILE, or default to
# config-default.sh.

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${KUBE_ROOT}/cluster/baresalt/${KUBE_CONFIG_FILE-"config-default.sh"}"
source "${KUBE_ROOT}/cluster/common.sh"

function json_val () {
    python -c 'import json,sys;obj=json.load(sys.stdin);print obj'$1'';
}

# Verify prereqs
function verify-prereqs {
    echo "===> verify-prereqs: TODO"
}

# Create a temp dir that'll be deleted at the end of this bash session.
#
# Vars set:
#   KUBE_TEMP
function ensure-temp-dir {
    if [[ -z ${KUBE_TEMP-} ]]; then
        KUBE_TEMP=$(mktemp -d -t kubernetes.XXXXXX)
        echo "KUBE_TEMP:${KUBE_TEMP}"
#        trap 'rm -rf "${KUBE_TEMP}"' EXIT
    fi
}

# Verify and find the various tar files that we are going to use on the server.
#
# Vars set:
#   SERVER_BINARY_TAR
#   SALT_TAR
function find-release-tars {
    SERVER_BINARY_TAR="${KUBE_ROOT}/server/kubernetes-server-linux-amd64.tar.gz"
    if [[ ! -f "$SERVER_BINARY_TAR" ]]; then
        SERVER_BINARY_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-server-linux-amd64.tar.gz"
    fi
    if [[ ! -f "$SERVER_BINARY_TAR" ]]; then
        echo "!!! Cannot find kubernetes-server-linux-amd64.tar.gz"
        exit 1
    fi

    SALT_TAR="${KUBE_ROOT}/server/kubernetes-salt.tar.gz"
    if [[ ! -f "$SALT_TAR" ]]; then
        SALT_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-salt.tar.gz"
    fi
    if [[ ! -f "$SALT_TAR" ]]; then
        echo "!!! Cannot find kubernetes-salt.tar.gz"
        exit 1
    fi
}

# Detect the information about the minions
#
# Assumed vars:
#   MINION_NAMES
#   ZONE
# Vars set:
#
function detect-minions () {
    echo "===> TODO: Detect Minions"
#    if [[ -z "$AZ_CS" ]]; then
#        verify-prereqs
#    fi
#    ssh_ports=($(eval echo "2200{1..$NUM_MINIONS}"))
#    for (( i=0; i<${#MINION_NAMES[@]}; i++)); do
#        MINION_NAMES[$i]=$(ssh -oStrictHostKeyChecking=no -i $AZ_SSH_KEY -p ${ssh_ports[$i]} $AZ_CS.cloudapp.net hostname -f)
#    done
}

# Detect the IP for the master
#
# Assumed vars:
#   MASTER_NAME
#   ZONE
# Vars set:
#   KUBE_MASTER
#   KUBE_MASTER_IP
function detect-master () {
    echo "===> TODO: Detect Master"
#    if [[ -z "$AZ_CS" ]]; then
#        verify-prereqs
#    fi
#
#    KUBE_MASTER=${MASTER_NAME}
#    KUBE_MASTER_IP="${AZ_CS}.cloudapp.net"
#    echo "Using master: $KUBE_MASTER (external IP: $KUBE_MASTER_IP)"
}

# Ensure that we have a password created for validating to the master.  Will
# read from kubeconfig current-context if available.
#
# Vars set:
#   KUBE_USER
#   KUBE_PASSWORD
function get-password {
  get-kubeconfig-basicauth
  if [[ -z "${KUBE_USER}" || -z "${KUBE_PASSWORD}" ]]; then
    KUBE_USER=admin
    KUBE_PASSWORD=$(python -c 'import string,random; print "".join(random.SystemRandom().choice(string.ascii_letters + string.digits) for _ in range(16))')
  fi
}

function provision-master {

    # Build up start up script for master
    export masternode=${1-}
    echo "--> Building up start up script for master"
    (
        echo "#!/bin/bash"
        echo "CA_CRT=\"$(cat ${KUBE_TEMP}/ca.crt)\""
        echo "SERVER_CRT=\"$(cat ${KUBE_TEMP}/server.crt)\""
        echo "SERVER_KEY=\"$(cat ${KUBE_TEMP}/server.key)\""
        echo "readonly MASTER_NAME='${MASTER_NAME}'"
        echo "readonly MASTER_IP='${masternode}'"
        echo "readonly NODES='${NODES}'"
        echo "readonly INSTANCE_PREFIX='${INSTANCE_PREFIX}'"
        echo "readonly NODE_INSTANCE_PREFIX='${INSTANCE_PREFIX}-minion'"
        echo "readonly MASTER_HTPASSWD='${htpasswd}'"
        echo "readonly SERVICE_CLUSTER_IP_RANGE='${SERVICE_CLUSTER_IP_RANGE}'"
        echo "readonly ADMISSION_CONTROL='${ADMISSION_CONTROL:-}'"
        echo "readonly KUBELET_TOKEN='${KUBELET_TOKEN:-}'"
        echo "readonly KUBE_PROXY_TOKEN='${KUBE_PROXY_TOKEN:-}'"
        echo "readonly KUBE_USER='${KUBE_USER:-}'"
        echo "readonly KUBE_PASSWORD='${KUBE_PASSWORD:-}'"
        echo "ENABLE_CLUSTER_MONITORING='${ENABLE_CLUSTER_MONITORING}'"
        echo "ENABLE_CLUSTER_LOGGING='${ENABLE_NODE_LOGGING:-false}'"
        echo "ENABLE_CLUSTER_UI='${ENABLE_CLUSTER_UI}'"
        echo "ENABLE_NODE_LOGGING='${ENABLE_NODE_LOGGING:-false}'"
        echo "LOGGING_DESTINATION='${LOGGING_DESTINATION:-}'"
        echo "ELASTICSEARCH_LOGGING_REPLICAS='${ELASTICSEARCH_LOGGING_REPLICAS:-1}'"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/common.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/create-dynamic-salt-files.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/download-release.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/salt-master.sh"
    ) > "${KUBE_TEMP}/master-start.sh"

    # remote login to MASTER and use sudo to configue k8s master
    ssh $SSH_OPTS -t $masternode "mkdir -p /var/cache/kubernetes-install"
    scp -r $SSH_OPTS ${KUBE_TEMP}/master-start.sh ${SERVER_BINARY_TAR} ${SALT_TAR} $masternode:/var/cache/kubernetes-install
    ssh -t $masternode "cd /var/cache/kubernetes-install; chmod +x master-start.sh; sudo ./master-start.sh"
}

function provision-minion {
    #Build up start up script for minions
    echo "--> Building up start up script for minions"
    currentnode=${1-}
    i=${2-}
    (
        echo "#!/bin/bash"
        echo "MASTER_NAME='${MASTER_NAME}'"
        echo "MASTER_IP='${masternode}'"
        echo "MINION_IP_RANGE='${MINION_IP_RANGES[$i]}'"
        echo "readonly KUBELET_TOKEN='${KUBELET_TOKEN:-}'"
        echo "readonly KUBE_PROXY_TOKEN='${KUBE_PROXY_TOKEN:-}'"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/common.sh"
        grep -v "^#" "${KUBE_ROOT}/cluster/baresalt/templates/salt-minion.sh"
    ) > "${KUBE_TEMP}/minion-start-${i}.sh"

        # remote login to MASTER and use sudo to configue k8s master
    ssh $SSH_OPTS -t $currentnode "mkdir -p /var/cache/kubernetes-install"
    scp -r $SSH_OPTS ${KUBE_TEMP}/minion-start-${i}.sh  $currentnode:/var/cache/kubernetes-install
    ssh -t $currentnode "cd /var/cache/kubernetes-install; chmod +x minion-start-${i}.sh; sudo ./minion-start-${i}.sh"
}



function provision-masterandminion {
    echo "TODO: provision MasterAndMinion"
}

# Instantiate a kubernetes cluster
#
# Assumed vars
#   KUBE_ROOT
#   <Various vars set in config file>
function kube-up {
    # Make sure we have the tar files staged on Azure Storage
    find-release-tars
    ensure-temp-dir

    get-password
    get-tokens
    python "${KUBE_ROOT}/third_party/htpasswd/htpasswd.py" \
        -b -c "${KUBE_TEMP}/htpasswd" "$KUBE_USER" "$KUBE_PASSWORD"
    local htpasswd
    htpasswd=$(cat "${KUBE_TEMP}/htpasswd")

    # Generate Cert(no openvpn -- need modify)
    echo "--> Generating openvpn certs"
    echo 01 > ${KUBE_TEMP}/ca.srl
    openssl genrsa -out ${KUBE_TEMP}/ca.key
    openssl req -new -x509 -days 1095 \
        -key ${KUBE_TEMP}/ca.key \
        -out ${KUBE_TEMP}/ca.crt \
        -subj "/CN=openvpn-ca"
    openssl genrsa -out ${KUBE_TEMP}/server.key
    openssl req -new \
        -key ${KUBE_TEMP}/server.key \
        -out ${KUBE_TEMP}/server.csr \
        -subj "/CN=server"
    openssl x509 -req -days 1095 \
        -in ${KUBE_TEMP}/server.csr \
        -CA ${KUBE_TEMP}/ca.crt \
        -CAkey ${KUBE_TEMP}/ca.key \
        -CAserial ${KUBE_TEMP}/ca.srl \
        -out ${KUBE_TEMP}/server.crt

    ii=0
    for i in ${NODES}; do
    {
        echo "NODE ID = ${i};  ROLE = ${ROLES[${ii}]} "
        if [ "${ROLES[${ii}]}" == "m" ]; then
            provision-master $i
        elif [ "${ROLES[${ii}]}" == "n" ]; then
            provision-minion $i $ii
        elif [ "${ROLES[${ii}]}" == "mn" ]; then
            provision-masterandminion
        else
            echo "unsupported role for ${i}. please check"
            exit 1
        fi
    }
    ((ii=ii+1))
     done

    detect-master

}

# Delete a kubernetes cluster
function kube-down {
    echo "===> TODO: Bringing Down CLuster"
}

function validate-cluster {
    echo "===> TODO: validate-cluster"
}


# Update a kubernetes cluster with latest source
#function kube-push {
#  detect-project
#  detect-master

# Make sure we have the tar files staged on Azure Storage
#  find-release-tars
#  upload-server-tars

#  (
#    echo "#! /bin/bash"
#    echo "mkdir -p /var/cache/kubernetes-install"
#    echo "cd /var/cache/kubernetes-install"
#    echo "readonly SERVER_BINARY_TAR_URL='${SERVER_BINARY_TAR_URL}'"
#    echo "readonly SALT_TAR_URL='${SALT_TAR_URL}'"
#    grep -v "^#" "${KUBE_ROOT}/cluster/azure/templates/common.sh"
#    grep -v "^#" "${KUBE_ROOT}/cluster/azure/templates/download-release.sh"
#    echo "echo Executing configuration"
#    echo "sudo salt '*' mine.update"
#    echo "sudo salt --force-color '*' state.highstate"
#   ) | gcutil ssh --project "$PROJECT" --zone "$ZONE" "$KUBE_MASTER" sudo bash

#  get-password

#  echo
#  echo "Kubernetes cluster is running.  The master is running at:"
#  echo
#  echo "  https://${KUBE_MASTER_IP}"
# echo
#  echo "The user name and password to use is located in ${KUBECONFIG:-$DEFAULT_KUBECONFIG}."
#  echo

#}

# -----------------------------------------------------------------------------
# Cluster specific test helpers used from hack/e2e-test.sh

# Execute prior to running tests to build a release if required for env.
#
# Assumed Vars:
#   KUBE_ROOT
function test-build-release {
    # Make a release
    "${KUBE_ROOT}/build/release.sh"
}

# SSH to a node by name ($1) and run a command ($2).
function ssh-to-node {
    local node="$1"
    local cmd="$2"
    ssh --ssh_arg "-o LogLevel=quiet" "${node}" "${cmd}"
}

# Restart the kube-proxy on a node ($1)
function restart-kube-proxy {
    ssh-to-node "$1" "sudo /etc/init.d/kube-proxy restart"
}

# Restart the kube-proxy on the master ($1)
function restart-apiserver {
    ssh-to-node "$1" "sudo /etc/init.d/kube-apiserver restart"
}

function get-tokens() {
  KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
}
