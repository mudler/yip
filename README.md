# :pushpin: yip


Simply applies a configuration to the system described with yaml files.


```yaml
stages:
   # "test" is the stage
   test:
     - systemd_firstboot:
         keymap: us
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
          content: |
                    #!/bin/sh
                    echo "test"
          permissions: 0777
          owner: 1000
          group: 100
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

A map of users and password to set. Passwords can be also encrypted.

```yaml
stages:
   default:
     - name: "Setup users"
       users: 
          bastion: "strongpassword"
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
