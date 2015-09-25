{% if grains.os == 'Ubuntu' %}
{% set environment_file = '/etc/default/flannel' %}
{{ environment_file }}:
  file.managed:
    - source: salt://flannel/default
    - template: jinja
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% set init_file = '/etc/init/flannel.conf' %}
{{ init_file }}:
  file.managed:
    - source: salt://flannel/flannel.conf.upstart
    - user: root
    - group: root
    - mode: 644
    - makedirs: true   

{% set initd_file = '/etc/init.d/flannel' %}
{{ initd_file }}:
  file.managed:
    - source: salt://flannel/flannel.upstart
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% else %}

# copy centos files.
{% set environment_file = '/opt/kubernetes/cfg/flannel' %}
{{ environment_file }}:
  file.managed:
    - source: salt://flannel/flanenl.service.conf
    - template: jinja
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% set service_file = '/usr/lib/systemd/system/flannel.service' %}
{{ service_file }}:
  file.managed:
    - source: salt://flannel/flannel.service
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% endif %}

# copy flannel binary to nodes
{% set binary_file = '/usr/bin/flannel' %}
{{ binary_file }}:
  file.managed:
    - source: salt://kube-bin/flannel
    - user: root
    - group: root
    - mode: 755
    - makedirs: true

{% set post_start_script = '/usr/bin/mk-docker-opts.sh' %}
{{ post_start_script }}:
  file.managed:
    - source: salt://kube-bin/mk-docker-opts.sh
    - user: root
    - group: root
    - mode: 755
    - makedirs: true

flannel:
  service:
    - name: flannel
    - running
    - reload: True
    - watch:
      - file: {{ environment_file }} 

