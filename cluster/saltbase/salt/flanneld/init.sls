{% if grains.os == 'Ubuntu' %}
{% set environment_file = '/etc/default/flanneld' %}
{{ environment_file }}:
  file.managed:
    - source: salt://flanneld/flanneld.conf
    - template: jinja
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% set init_file = '/etc/init/flanneld.conf' %}
{{ init_file }}:
  file.managed:
    - source: salt://flanneld/flanneld.conf.upstart
    - user: root
    - group: root
    - mode: 644
    - makedirs: true   

{% set initd_file = '/etc/init.d/flanneld' %}
{{ initd_file }}:
  file.managed:
    - source: salt://flanneld/flanneld.upstart
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

/etc/init/mk-docker-opts.conf:
  file.managed:
    - source: salt://flanneld/mk-docker-opts.conf.upstart
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

/etc/init/set_docker_bridge.conf:
  file.managed:
    - source: salt://flanneld/set_docker_bridge.conf
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% else %}

# copy centos files.
{% set environment_file = '/opt/kubernetes/cfg/flanneld' %}
{{ environment_file }}:
  file.managed:
    - source: salt://flanneld/flanneld.conf
    - template: jinja
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% set service_file = '/usr/lib/systemd/system/flanneld.service' %}
{{ service_file }}:
  file.managed:
    - source: salt://flanneld/flanneld.service
    - user: root
    - group: root
    - mode: 644
    - makedirs: true

{% endif %}

# copy flanneld binary to nodes
{% set binary_file = '/usr/bin/flanneld' %}
{{ binary_file }}:
  file.managed:
    - source: salt://kube-bins/flanneld
    - user: root
    - group: root
    - mode: 755
    - makedirs: true

{% set post_start_script = '/usr/bin/mk-docker-opts.sh' %}
{{ post_start_script }}:
  file.managed:
    - source: salt://kube-bins/mk-docker-opts.sh
    - user: root
    - group: root
    - mode: 755
    - makedirs: true

flanneld:
  service:
    - name: flanneld
    - running
    - reload: True
    - watch:
      - file: {{ binary_file }} 

