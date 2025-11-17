# :pushpin: yip


Simply applies a configuration to the system described with yaml files.


```yaml
stages:
   # "test" is the stage
   test:
     - systemd_firstboot:
         keymap: us
     - files:
        - path: /tmp/bar
          content: |
                    test
          permissions: 0777
          owner: 1000
          group: 100
       if: "[ ! -e /tmp/bar ]"
     - files:
        - path: /tmp/foo
          content: |
                    test
          permissions: 0777
          owner: 1000
          group: 100
       commands:
        - echo "test"
       modules:
       - nvidia
       environment:
         FOO: "bar"
       systctl:
         debug.exception-trace: "0"
       hostname: "foo"
       systemctl:
         enable:
         - foo
         disable:
         - bar
         start:
         - baz
         mask:
         - foobar
       authorized_keys:
          user:
          - "github:mudler"
          - "ssh-rsa ...."
       dns:
         path: /etc/resolv.conf
         nameservers:
         - 8.8.8.8
       ensure_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "pass"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
       delete_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "pass"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
      datasource:
        providers:
          - "digitalocean"
          - "aws"
          - "gcp"
        path: "/usr/local/etc"
```

- Simple
- Small scope, pluggable, extensible

Yip uses a simple, yet powerful distro-agnostic cloud-init style format for the definition.

```bash
$> yip -s test yip1.yaml yip2.yaml
$> yip -s test https://..
```
---

That's it! by default `yip` uses the default stage and the `default` executor, but you can customize its execution.


```
yip loads cloud-init style yamls and applies them in the system.

For example:

        $> yip -s initramfs https://<yip.yaml> /path/to/disk <definition.yaml> ...
        $> yip -s initramfs <yip.yaml> <yip2.yaml> ...
        $> cat def.yaml | yip -

Usage:
  yip [flags]

Flags:
  -e, --executor string   Executor which applies the config (default "default")
  -h, --help              help for yip
  -s, --stage string      Stage to apply (default "default")
```


## How it works


Yip works in *stages*. You can define *stages* that you can decide to run and apply in various ways and in a different enviroment (that's why *stages*).  

A stage is just a list of steps, for example the following:

```yaml
stages:
   default:
     - files:
        - path: /tmp/bar
          content: |
                    #!/bin/sh
                    echo "test"
          permissions: 0777
          owner: 1000
          group: 100
       commands:
        - /tmp/bar
```

writes a `/tmp/bar` file during the `default` stage and will also run it afterwards. 

Now we can execute it:

```bash
$> cat myfile.yaml | yip -s default -
```

As `yip` by default runs the `default` stage we could have just run:

```bash
$> cat myfile.yaml | yip -
```

A yaml file can define multiple stages, which can be run from the `cli` with `-s`. Each stage is defined under `stages`, and in each stage are defined a list of `steps` to execute.

`Yip` will execute the steps and report failures. It will exit non-zero if one of the steps failed executing. It will, however, keep running all the detected `yipfiles` and stages.

## Compatibility with Cloud Init format

A subset of the official [cloud-config spec](http://cloudinit.readthedocs.org/en/latest/topics/format.html#cloud-config-data) is implemented by yip. 

If a yaml file starts with `#cloud-config` it is parsed as a standard cloud-init, associated it to the yip `boot` stage. For example:

```yaml
#cloud-config
users:
- name: "bar"
  passwd: "foo"
  groups: "users"
  ssh_authorized_keys:
  - faaapploo
ssh_authorized_keys:
  - asdd
runcmd:
- foo
hostname: "bar"
write_files:
- encoding: b64
  content: CiMgVGhpcyBmaWxlIGNvbnRyb2xzIHRoZSBzdGF0ZSBvZiBTRUxpbnV4
  path: /foo/bar
  permissions: "0644"
  owner: "bar"
```

To execute it with yip, run `yip -s boot cloud-config.yaml`.


## Node-data interpolation

`yip` interpolates host data retrieved by [sysinfo](https://github.com/zcalusic/sysinfo#sample-output) and are templated in the commands, file and entities  fields.

This means that templating like the following is possible:

```yaml
stages:
  foo:
  - name: "echo"
    commands:
    - echo "{{.Values.node.hostname}}"

name: "Test yip!"
```

## Filtering stages by node hostname

`yip` can skip stages based on the node hostname:


```yaml
stages:
  foo:
  - name: "echo"
    commands:
    - echo hello
    node: "hostname" # Node hostname

name: "Test yip!"
```

## Filtering stages with if statement

`yip` can skip stages based on if statements:


```yaml
stages:
  foo:
  - name: "echo"
    commands:
    - echo hello
    if: "cat /proc/cmdline | grep debug"

name: "Test yip!"
```

The expression inside the if will be evaluated in bash and, if specified, the stage gets executed only if the condition returns successfully (exit 0).

## Filtering stages with only_os and only_os_version statement

`yip` can skip stages based on the OS and OS version:

Use the `only_os` and `only_os_version` fields to specify the OS and OS version where the stage should be executed. They can be used together or separately and mixed with the normal `if` statement to provide several filtering criteria.

Notice that both fields are compiled as a regex, so you can use regex patterns to match the OS and OS version.

```yaml
stages:
  foo:
  - name: "echo ubuntu"
    commands:
    - echo hello
    only_os: "ubuntu"
  - name: "echo ubuntu and opensuse-leap"
    commands:
       - echo hello
    only_os: "ubuntu|opensuse-leap"
  - name: "echo everything but ubuntu"
    commands:
       - echo hello
    only_os: "^(?!ubuntu).*"
  - name: "echo ubuntu 20.04"
    commands:
       - echo hello
    only_os: "ubuntu"
    only_os_version: "20.04"
  - name: "echo ubuntu 20.04 or 22.04"
    commands:
       - echo hello
    only_os: "ubuntu"
    only_os_version: "20.04|22.04"

name: "Test yip!"
```


## Filtering stages with if_files statement

`yip` can skip stages based on the existence of files:

```yaml
stages:
    foo:
    - name: "echo"
      commands:
      - echo hello
      if_files:
        any:
          - /etc/os-release
          - /tmp/foo
        all:
          - /etc/passwd
        none:
          - /tmp/bar
```

The `if_files` field accepts three subfields: `any`, `all`, and `none`. Each subfield takes a list of file paths.
- `any`: The stage will execute if at least one of the specified files exists.
- `all`: The stage will execute only if all the specified files exist.
- `none`: The stage will execute only if none of the specified files exist.

You can use any combination of these subfields to create complex file existence conditions for stage execution.


## Configuration reference

Below is a reference of all keys available in the cloud-init style files.


### `stages.<stageID>.[<stepN>].name`

A description of the stage step. Used only when printing output to console.

### `stages.<stageID>.[<stepN>].files`

A list of files to write to disk.

```yaml
stages:
   default:
     - files:
        - path: /tmp/bar
          encoding: "b64" # "base64", "gz", "gzip", "gz+base64", "gzip+base64", "gz+b64", "gzip+b64"
          content: |
                    #!/bin/sh
                    echo "test"
          permissions: 0777
          owner: 1000
          group: 100
          # or
          # owner_string: "user:group", or "user"
```

### `stages.<stageID>.[<stepN>].downloads`

A list of http urls to download and write to disk.

```yaml
stages:
   default:
     - downloads:
        - path: /tmp/bar
          url: ""
          timeout: 0
          permissions: 0777
          owner: 1000
          group: 100
          # or
          # owner_string: "user:group", or "user"
```

### `stages.<stageID>.[<stepN>].directories`

A list of directories to be created on disk. Runs before `files`.

```yaml
stages:
   default:
     - name: "Setup folders"
       directories: 
       - path: "/etc/foo"
         permissions: 0600
         owner: 0
         group: 0
```

### `stages.<stageID>.[<stepN>].dns`

A way to configure the `/etc/resolv.conf` file.

```yaml
stages:
   default:
     - name: "Setup dns"
       dns: 
         nameservers:
         - 8.8.8.8
         - 1.1.1.1
         search:
         - foo.bar
         options:
         - ..
         path: "/etc/resolv.conf.bak"
```
### `stages.<stageID>.[<stepN>].hostname`

A string representing the machine hostname. It sets it in the running system, updates `/etc/hostname` and adds the new hostname to `/etc/hosts`.

```yaml
stages:
   default:
     - name: "Setup hostname"
       hostname: "foo"
```
### `stages.<stageID>.[<stepN>].sysctl`

Kernel configuration. It sets `/proc/sys/<key>` accordingly, similarly to `sysctl`.

```yaml
stages:
   default:
     - name: "Setup exception trace"
       systctl:
         debug.exception-trace: "0"
```

### `stages.<stageID>.[<stepN>].authorized_keys`

A list of SSH authorized keys that should be added for each user. 
SSH keys can be obtained from GitHub user accounts by using the format github:${USERNAME},  similarly for Gitlab with gitlab:${USERNAME}.

```yaml
stages:
   default:
     - name: "Setup exception trace"
       authorized_keys:
         mudler:
         - github:mudler
         - ssh-rsa: ...
```

### `stages.<stageID>.[<stepN>].node`

If defined, the node hostname where this stage has to run, otherwise it skips the execution. The node can be also a regexp in the Golang format.

```yaml
stages:
   default:
     - name: "Setup logging"
       node: "bastion"
```

### `stages.<stageID>.[<stepN>].users`

A map of users and user info to set. Passwords can be also encrypted.

The `users` parameter adds or modifies the specified list of users. Each user is an object which consists of the following fields. Each field is optional and of type string unless otherwise noted.
In case the user is already existing, the password only will be overwritten.

- **name**: Required. Login name of user
- **gecos**: GECOS comment of user
- **passwd**: Hash of the password to use for this user. Unencrypted strings supported too.
- **homedir**: User's home directory. Defaults to /home/*name*
- **no-create-home**: Boolean. Skip home directory creation.
- **primary-group**: Default group for the user. Defaults to a new group created named after the user.
- **groups**: Add user to these additional groups
- **no-user-group**: Boolean. Skip default group creation.
- **ssh-authorized-keys**: List of public SSH keys to authorize for this user
- **system**: Create the user as a system user. No home directory will be created.
- **no-log-init**: Boolean. Skip initialization of lastlog and faillog databases.
- **shell**: User's login shell.

```yaml
stages:
   default:
     - name: "Setup users"
       users: 
          bastion: 
            passwd: "strongpassword"
            homedir: "/home/foo
```

### `stages.<stageID>.[<stepN>].ensure_entities`

A `user` or a `group` in the [entity](https://github.com/mudler/entities) format to be configured in the system

```yaml
stages:
   default:
     - name: "Setup users"
       ensure_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "x"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
```
### `stages.<stageID>.[<stepN>].delete_entities`

A `user` or a `group` in the [entity](https://github.com/mudler/entities) format to be pruned from the system

```yaml
stages:
   default:
     - name: "Setup users"
       delete_entities:
       -  path: /etc/passwd
          entity: |
                  kind: "user"
                  username: "foo"
                  password: "x"
                  uid: 0
                  gid: 0
                  info: "Foo!"
                  homedir: "/home/foo"
                  shell: "/bin/bash"
```
### `stages.<stageID>.[<stepN>].modules`

A list of kernel modules to load.

```yaml
stages:
   default:
     - name: "Setup users"
       modules:
       - nvidia
```
### `stages.<stageID>.[<stepN>].systemctl`

A list of systemd services to `enable`, `disable`, `mask` or `start`.

A list of overrides to apply to the service files witht he following fields:
 - `service`: The service name to add the override for. Required. `.service` extension is appended to the name if not provided.
   - `name`: The name of the override file. Optional, if not provided it will be named `override-yip.conf`. `.conf` extension is appended to the name if not provided.
 - `content`: The content of the override file.

Overrides don't check if the service is enabled or disabled, or even if it exists, it will apply the override anyway.

```yaml
stages:
   default:
     - name: "Setup users"
       systemctl:
         enable:
          - systemd-timesyncd
          - cronie
         mask:
          - purge-kernels
         disable:
          - crond
         start:
          - cronie
         overrides:
            - service: "systemd-timesyncd"
              name: "override-custom.conf"
              content: |
                [Service]
                ExecStart=
                ExecStart=/usr/lib/systemd/systemd-timesyncd
```
### `stages.<stageID>.[<stepN>].environment`

A map of variables to write in `/etc/environment`, or otherwise specified in `environment_file`

```yaml
stages:
   default:
     - name: "Setup users"
       environment:
         FOO: "bar"
```
### `stages.<stageID>.[<stepN>].environment_file`

A string to specify where to set the environment file

```yaml
stages:
   default:
     - name: "Setup users"
       environment_file: "/home/user/.envrc"
       environment:
         FOO: "bar"
```
### `stages.<stageID>.[<stepN>].timesyncd`

Sets the `systemd-timesyncd` daemon file (`/etc/system/timesyncd.conf`) file accordingly. The documentation for `timesyncd` and all the options can be found [here](https://www.freedesktop.org/software/systemd/man/timesyncd.conf.html).

```yaml
stages:
   default:
     - name: "Setup NTP"
       systemctl:
         enable:
         - systemd-timesyncd
       timesyncd: 
          NTP: "0.pool.org foo.pool.org"
          FallbackNTP: ""
          ...
```

### `stages.<stageID>.[<stepN>].systemd_firstboot`

Runs `systemd-firstboot` with the given map

```yaml
stages:
   default:
     - name: "Setup Locale"
       systemd_firstboot:
         keymap: us
```

### `stages.<stageID>.[<stepN>].commands`

A list of arbitrary commands to run after file writes and directory creation.

```yaml
stages:
   default:
     - name: "Setup something"
       commands:
         - echo 1 > /bar
```

### `stages.<stageID>.[<stepN>].datasource`

Sets to fetch user data from the specified cloud providers. It iterates
through the list of providers and the first one that succeeds to
extract some user data is the one being used. It populates provider
specific data into `/run/config` folder and the custom user data is stored
into the provided path.


```yaml
stages:
   default:
     - name: "Fetch cloud provider's user data"
       datasource:
         providers:
           - "aws"
           - "digitalocean"
         path: "/etc/cloud-data"
```

### `stages.<stageID>.[<stepN>].layout`

Sets additional partitions on disk free space, if any, and/or expands the last
partition. All sizes are expressed in MiB only and default value of `size: 0`
means all available free space in disk. This plugin is useful to be used in
oem images where the default partitions might not suit the actual disk geometry.


```yaml
stages:
   default:
     - name: "Repart disk"
       layout:
         device:
           # It will partition a device including the given filesystem label
           # or partition label (filesystem label matches first) or the device
           # provided in 'path'. The label check has precedence over path when
           # both are provided.
           # init_disk set to true is used to initialize the disk partition table with an empty GPT table. This
           # is useful when we want to initialize the disk from scratch to add our own partitions.
           # WARNING: This will destroy all data in the disk! So only run it when you are dealing with a new disk or
           # you are sure you want to wipe out all data in the disk.
           # disk_name is optional and used to set deterministic GUID disk name when init_disk is true. This allows for
           # reproducible disk GUIDs across multiple runs and easy identification of disk by GUID.
           # The GUID generated is a V5 UUID based on the disk_name provided under the DNS namespace.
           # Not setting it will default to a GUID based on the YIP_DISK name
           label: COS_RECOVERY
           path: /dev/sda
           init_disk: true
           disk_name: "MYDISK""
         # Only last partition can be expanded and it happens before any other
         # partition is added.
         expand_partition:
           size: 4096 #  size: 0 means all available free space
         add_partitions:
           - fsLabel: COS_STATE
             size: 8192
             # No partition label is applied if omitted
             pLabel: state
           - fsLabel: COS_PERSISTENT
             # default filesystem is ext2 if omitted
             filesystem: ext4
```

### `stages.<stageID>.[<stepN>].unpack_images`

Unpacks a list of OCI images to disk.
Accepts a list of `source` and `target` paths.
The `source` is the image to unpack, and the `target` is the path where the image will be unpacked.
Optially a `platform` can be specified to unpack the image for a specific platform. By default it will unpack the image for the current platform.

```yaml
stages:
   default:
     - name: "Unpack images"
       unpack_images:
         - source: "quay.io/luet/base:latest"
           target: "/usr/local/luet/"
         - source: "rancher/k3s:latest"
           target: "/usr/local/k3s-arm64"
           platform: "linux/arm64"
```