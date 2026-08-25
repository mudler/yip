# :pushpin: yip

Applies a configuration described in YAML to the system. Distro-agnostic, cloud-init style, small scope, pluggable.

```yaml
name: "Example"
stages:
  default:
    - name: "Write a file and run it"
      files:
        - path: /tmp/hello.sh
          content: |
            #!/bin/sh
            echo "hello from yip"
          permissions: 0755
      commands:
        - /tmp/hello.sh
```

```bash
$> yip -s default file.yaml
$> yip -s default https://example.com/file.yaml
$> cat file.yaml | yip -
```

## Table of contents

- [How it works](#how-it-works)
- [CLI usage](#cli-usage)
- [Cloud-init compatibility](#cloud-init-compatibility)
- [Node-data interpolation](#node-data-interpolation)
- [Stage filtering](#stage-filtering)
- [Configuration reference](#configuration-reference)
  - [Metadata and flow control](#metadata-and-flow-control)
  - [Files, directories and downloads](#files-directories-and-downloads)
  - [Users, groups and SSH](#users-groups-and-ssh)
  - [System configuration](#system-configuration)
  - [Services](#services)
  - [Packages](#packages)
  - [Disk and images](#disk-and-images)
  - [Cloud data sources](#cloud-data-sources)
  - [Commands](#commands)

## How it works

Yip runs *stages*. A stage is a named list of *steps*, and each step is a map of fields that trigger built-in plugins. For example:

```yaml
stages:
  default:
    - files:
        - path: /tmp/bar
          content: "hello"
          permissions: 0644
      commands:
        - cat /tmp/bar
```

The step above triggers two plugins in the same step: `files` (writes `/tmp/bar`) and `commands` (runs `cat`). Within a single step plugins run in a fixed order — you do not need to split them across steps to guarantee ordering.

A YAML file can define multiple stages, and yip can be told which one to run with `-s <stage>`. Multiple files can be passed on the command line; their stages are merged before execution.

If any step fails yip exits non-zero, but it keeps running the remaining steps and files.

## CLI usage

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

Sources can be local paths, HTTP(S) URLs, or `-` for stdin.

## Cloud-init compatibility

A subset of the official [cloud-config spec](http://cloudinit.readthedocs.org/en/latest/topics/format.html#cloud-config-data) is understood. If a file starts with `#cloud-config` it is parsed as cloud-init and mapped to the yip `boot` stage:

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

Run with `yip -s boot cloud-config.yaml`.

## Node-data interpolation

Host data collected by [sysinfo](https://github.com/zcalusic/sysinfo#sample-output) is exposed to templates in `commands`, `files`, and entity fields:

```yaml
stages:
  default:
    - name: "echo hostname"
      commands:
        - echo "{{.Values.node.hostname}}"

name: "Test yip!"
```

## Stage filtering

Each step supports a set of predicates that decide whether it runs. Predicates are AND-ed: all of them must pass for the step to execute.

### `if`

Evaluates the expression as a shell command. The step runs only when the command exits `0`.

```yaml
stages:
  default:
    - name: "debug boot only"
      if: "cat /proc/cmdline | grep debug"
      commands:
        - echo "debug enabled"
```

### `if_files`

Runs (or skips) the step based on file existence. Accepts three optional sub-lists:

- `any`: at least one of the files exists.
- `all`: all of the files exist.
- `none`: none of the files exist.

Combine them freely — the step runs when every provided sub-condition is satisfied.

```yaml
stages:
  default:
    - name: "only on Linux hosts that never saw yip"
      if_files:
        any:
          - /etc/os-release
        all:
          - /etc/passwd
        none:
          - /var/lib/yip/done
      commands:
        - touch /var/lib/yip/done
```

### `node`

Runs the step only on hosts whose hostname matches the pattern. Accepts a Go regexp.

```yaml
stages:
  default:
    - node: "bastion.*"
      commands:
        - echo "runs on bastion hosts only"
```

### `only_os` / `only_os_version`

Matches the `ID` and `VERSION_ID` fields from `/etc/os-release`. Both are compiled as Go regexps.

```yaml
stages:
  default:
    - name: "ubuntu or opensuse-leap"
      only_os: "ubuntu|opensuse-leap"
      commands:
        - echo hello
    - name: "everything but ubuntu"
      only_os: "^(?!ubuntu).*"
      commands:
        - echo hello
    - name: "ubuntu 20.04 or 22.04"
      only_os: "ubuntu"
      only_os_version: "20.04|22.04"
      commands:
        - echo hello
```

### `only_arch`

Matches `runtime.GOARCH` (`amd64`, `arm64`, `riscv64`, ...) against a Go regexp.

```yaml
stages:
  default:
    - name: "only on arm64"
      only_arch: "arm64"
      commands:
        - echo "arm64!"
    - name: "arm64 or amd64"
      only_arch: "arm64|amd64"
      commands:
        - echo hello
```

### `only_service_manager`

Runs the step only when the host uses the given service manager. Supported values: `systemd`, `openrc`. Detection is done by probing for the corresponding binaries; a system where both are present is treated as ambiguous and the step is skipped.

```yaml
stages:
  default:
    - name: "enable service under systemd only"
      only_service_manager: "systemd"
      systemctl:
        enable:
          - my-service
```

## Configuration reference

Below is a reference of all keys available in a step.

### Metadata and flow control

#### `stages.<stageID>.[<stepN>].name`

Human-readable description of the step. Used only when printing progress to the console.

```yaml
stages:
  default:
    - name: "Setup logging"
      commands:
        - echo "started"
```

#### `stages.<stageID>.[<stepN>].after`

Declares that this step depends on other named steps. Yip uses this to build a dependency graph within a stage. The referenced names must match the `name` field of other steps in the same stage.

```yaml
stages:
  default:
    - name: "write config"
      files:
        - path: /etc/myapp.conf
          content: "key=value"
    - name: "start service"
      after:
        - name: "write config"
      commands:
        - systemctl start myapp
```

### Files, directories and downloads

#### `stages.<stageID>.[<stepN>].directories`

Creates directories on disk. Runs before `files` so that file writes into fresh directories succeed in one step.

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

#### `stages.<stageID>.[<stepN>].files`

Writes files to disk.

Supported `encoding` values: `b64`, `base64`, `gz`, `gzip`, `gz+base64`, `gzip+base64`, `gz+b64`, `gzip+b64`. Owner and group can be given as numeric uid/gid (`owner`, `group`) or as a string (`ownerstring: "user:group"` or `"user"`).

```yaml
stages:
  default:
    - files:
        - path: /tmp/bar
          content: |
            #!/bin/sh
            echo "test"
          permissions: 0755
          owner: 1000
          group: 100
        - path: /etc/motd
          encoding: b64
          content: SGVsbG8gd29ybGQK
          ownerstring: "root:root"
```

#### `stages.<stageID>.[<stepN>].downloads`

Downloads files over HTTP(S) and writes them to disk. `timeout` is in seconds; `0` means no timeout.

```yaml
stages:
  default:
    - downloads:
        - path: /usr/local/bin/tool
          url: "https://example.com/tool"
          timeout: 30
          permissions: 0755
          ownerstring: "root:root"
```

#### `stages.<stageID>.[<stepN>].git`

Clones a git repository into `path`. Supports HTTPS with username/password and SSH with a private key. `branch_only: true` fetches only the specified branch (single-branch checkout).

```yaml
stages:
  default:
    - name: "Clone config repo"
      git:
        url: "https://github.com/example/config.git"
        path: /opt/config
        branch: "main"
        branch_only: true
        auth:
          username: "gituser"
          password: "gitpassword"

    - name: "Clone via SSH key"
      git:
        url: "git@github.com:example/private.git"
        path: /opt/private
        branch: "main"
        auth:
          private_key: |
            -----BEGIN OPENSSH PRIVATE KEY-----
            ...
            -----END OPENSSH PRIVATE KEY-----
          # optional; set to true to skip host key verification
          insecure: true
```

### Users, groups and SSH

#### `stages.<stageID>.[<stepN>].users`

Adds or modifies users. Each entry is keyed by username and takes the following fields. Every field is optional and of type string unless noted otherwise. When a user already exists, only the password is updated.

- **name**: Login name. Defaults to the map key.
- **passwd**: Password hash. Plain strings are accepted as well.
- **gecos**: GECOS comment.
- **homedir**: Home directory. Defaults to `/home/<name>`.
- **no_create_home**: Boolean. Skip home directory creation.
- **primary_group**: Primary group name. Defaults to a new group named after the user.
- **groups**: List of additional groups the user belongs to.
- **no_user_group**: Boolean. Skip creating the per-user primary group.
- **ssh_authorized_keys**: List of public SSH keys to add for this user. Entries in `github:<name>` / `gitlab:<name>` form are resolved by fetching the user's public keys from the corresponding service.
- **system**: Boolean. Create a system account (no home directory).
- **no_log_init**: Boolean. Skip initializing lastlog and faillog.
- **shell**: Login shell.
- **lock_passwd**: Boolean. Lock the account password (equivalent to setting `!` in `/etc/shadow`).
- **uid**: Numeric user id. When set, the user is created with this exact uid — no free-uid search and no home-directory-owner reuse. Useful on immutable systems where `/etc/passwd` is regenerated on every boot but `/home` is persisted, to keep file ownership stable.
- **gid**: Numeric group id for the user's own primary group. When set (and `primary_group` is empty), the primary group is created with this exact gid. If `primary_group` is set too, that group's existing gid is used and `gid` is ignored.
- **subuid**: Subordinate user ID range in `start:count` format. Yip creates or replaces the user's entry in `/etc/subuid`.
- **subgid**: Subordinate group ID range in `start:count` format. Yip creates or replaces the user's entry in `/etc/subgid`.

```yaml
stages:
  default:
    - name: "Setup users"
      users:
        bastion:
          passwd: "strongpassword"
          homedir: "/home/bastion"
          shell: "/bin/bash"
          groups:
            - wheel
          ssh_authorized_keys:
            - github:mudler
        svc-app:
          uid: "12345"
          subuid: "100000:65536"
          subgid: "100000:65536"
          system: true
          shell: "/usr/sbin/nologin"
```

#### `stages.<stageID>.[<stepN>].authorized_keys`

Adds SSH keys to the `authorized_keys` file of an existing user. Entries starting with `github:` or `gitlab:` are fetched from those services.

```yaml
stages:
  default:
    - name: "Provision SSH keys"
      authorized_keys:
        mudler:
          - github:mudler
          - "ssh-rsa AAAA..."
        root:
          - "ssh-ed25519 AAAA..."
```

#### `stages.<stageID>.[<stepN>].ensure_entities`

Adds a user or group in the [entities](https://github.com/mudler/entities) format. Useful when you need full control over the `/etc/passwd`, `/etc/shadow`, or `/etc/group` row.

```yaml
stages:
  default:
    - name: "Ensure system user"
      ensure_entities:
        - path: /etc/passwd
          entity: |
            kind: "user"
            username: "foo"
            password: "x"
            uid: 1500
            gid: 1500
            info: "Foo service user"
            homedir: "/home/foo"
            shell: "/bin/bash"
```

#### `stages.<stageID>.[<stepN>].delete_entities`

Removes a user or group entity from the system.

```yaml
stages:
  default:
    - name: "Drop legacy user"
      delete_entities:
        - path: /etc/passwd
          entity: |
            kind: "user"
            username: "legacy"
```

### System configuration

#### `stages.<stageID>.[<stepN>].hostname`

Sets the machine hostname on the running system, writes it to `/etc/hostname`, and updates `/etc/hosts`.

```yaml
stages:
  default:
    - hostname: "node-01"
```

#### `stages.<stageID>.[<stepN>].dns`

Writes `/etc/resolv.conf` (or the file specified in `path`).

```yaml
stages:
  default:
    - name: "Setup DNS"
      dns:
        nameservers:
          - 8.8.8.8
          - 1.1.1.1
        search:
          - example.com
        options:
          - "timeout:2"
        path: "/etc/resolv.conf"
```

#### `stages.<stageID>.[<stepN>].sysctl`

Sets kernel parameters via `/proc/sys/<key>`, equivalent to `sysctl -w`.

```yaml
stages:
  default:
    - name: "Kernel tuning"
      sysctl:
        net.ipv4.ip_forward: "1"
        debug.exception-trace: "0"
```

#### `stages.<stageID>.[<stepN>].modules`

Loads kernel modules.

```yaml
stages:
  default:
    - name: "GPU driver"
      modules:
        - nvidia
        - nvidia_uvm
```

#### `stages.<stageID>.[<stepN>].environment`

Writes variables to `/etc/environment` (or the file specified in `environment_file`).

```yaml
stages:
  default:
    - environment:
        FOO: "bar"
        HTTP_PROXY: "http://proxy:3128"
```

#### `stages.<stageID>.[<stepN>].environment_file`

Overrides the file written by `environment`. Use it to target a user-specific env file such as `~/.envrc` instead of the system-wide `/etc/environment`.

```yaml
stages:
  default:
    - environment_file: "/home/user/.envrc"
      environment:
        FOO: "bar"
```

#### `stages.<stageID>.[<stepN>].timesyncd`

Writes a drop-in file at `/etc/systemd/timesyncd.conf.d/10-yip.conf` (under the `[Time]` section). Any key from [timesyncd.conf](https://www.freedesktop.org/software/systemd/man/timesyncd.conf.html) can be set.

```yaml
stages:
  default:
    - name: "Setup NTP"
      systemctl:
        enable:
          - systemd-timesyncd
      timesyncd:
        NTP: "0.pool.ntp.org 1.pool.ntp.org"
        FallbackNTP: ""
```

#### `stages.<stageID>.[<stepN>].systemd_firstboot`

Runs `systemd-firstboot` with the given map. Keys map to `--<key>=<value>` arguments (`keymap`, `locale`, `timezone`, ...).

```yaml
stages:
  default:
    - systemd_firstboot:
        keymap: us
        locale: en_US.UTF-8
        timezone: Europe/Madrid
```

### Services

#### `stages.<stageID>.[<stepN>].systemctl`

Manages systemd unit state and optionally drops override files. All lists are optional.

- **enable**: units to enable.
- **disable**: units to disable.
- **start**: units to start now.
- **mask**: units to mask.
- **overrides**: list of drop-in override files. Each entry:
  - **service**: unit name (`.service` is appended if missing). Required.
  - **name**: override file name. Optional, defaults to `override-yip.conf`. `.conf` is appended if missing.
  - **content**: full content of the override file.

Overrides are written even if the unit is not enabled or does not exist yet.

```yaml
stages:
  default:
    - name: "Configure services"
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

### Packages

#### `stages.<stageID>.[<stepN>].packages`

Installs, removes, refreshes, or upgrades packages using the host package manager. The distro is detected from `/etc/os-release`; supported managers: `apt-get` (Debian/Ubuntu), `dnf` (Fedora/RHEL/Rocky/Alma/Oracle/CentOS/openEuler), `zypper` (openSUSE/SLE), `pacman` (Arch), `apk` (Alpine).

Operation order is: `refresh` → `upgrade` → `install` → `remove`.

```yaml
stages:
  default:
    - name: "Base packages"
      packages:
        refresh: true
        upgrade: false
        install:
          - vim
          - curl
        remove:
          - nano
```

#### `stages.<stageID>.[<stepN>].package_pins`

Best-effort version pinning applied before package installs. The exact mechanism depends on the detected package manager:

- **apt-get**: writes `/etc/apt/preferences.d/99-yip-pins` with `Pin-Priority: 1001`.
- **dnf**: writes `/etc/dnf/plugins/versionlock.conf` + a `versionlock.list`. Full NEVRA is resolved with `dnf repoquery` when possible, otherwise a raw pattern is used.
- **zypper**: runs `zypper addlock name=version`, falling back to `addlock name` if the version-scoped lock is rejected.
- **apk**: rewrites `/etc/apk/world` with `name=version` entries.
- **pacman**: not supported; the step is skipped with a warning.

```yaml
stages:
  default:
    - name: "Pin kernel and container runtime"
      package_pins:
        linux-image-generic: "5.15.0-91.101"
        containerd.io: "1.7.11"
```

### Disk and images

#### `stages.<stageID>.[<stepN>].layout`

Repartitions a disk: adds partitions in the trailing free space and/or expands the last partition. Sizes are always in MiB; `size: 0` means "use all remaining free space".

Device targeting: the target disk is picked by matching (in order) `label` (filesystem or partition label) or `path`. `path` also accepts a `script://<command>` form — the command is executed and its trimmed stdout is used as the device path, useful when the target is not known ahead of time.

If `init_disk: true` the disk is wiped and a fresh GPT is written. **Destructive** — only use on a disk you fully control. `disk_name` (optional) makes the GPT GUID deterministic (UUIDv5 derived from the name under the URL namespace, `uuid.NamespaceURL`).

Only the last partition can be expanded, and expansion happens before any new partition is added. Expansion only grows the partition — it never shrinks it.

For `add_partitions`:

- **size** (required): MiB. `0` means all remaining free space.
- **fsLabel**: filesystem label. Optional.
- **pLabel**: partition label. Optional but recommended — if a partition with the same `pLabel` already exists it is left untouched, otherwise data may be lost.
- **filesystem**: `ext2` (default), `ext3`, `ext4`, `fat`, `vfat`, `fat16`, `fat32`, `xfs`, `btrfs`, `swap`, or `noformat` / `-` / `none` to create the partition without formatting.
- **bootable**: boolean. Sets the bootable flag and the corresponding partition GUID type.

```yaml
stages:
  default:
    - name: "Repart disk"
      layout:
        device:
          label: COS_RECOVERY
          path: /dev/sda
          init_disk: true
          disk_name: "MYDISK"
        expand_partition:
          size: 4096  # 0 for all remaining free space
        add_partitions:
          - fsLabel: COS_STATE
            size: 8192
            pLabel: state
          - fsLabel: COS_PERSISTENT
            filesystem: ext4
            size: 0
          - fsLabel: COS_GRUB
            size: 512
            filesystem: fat32
            bootable: true
          - pLabel: raw_data
            size: 1024
            filesystem: noformat
```

#### `stages.<stageID>.[<stepN>].unpack_images`

Unpacks OCI images onto the filesystem. `platform` is optional and defaults to the host platform.

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

### Cloud data sources

#### `stages.<stageID>.[<stepN>].datasource`

Fetches user data from the specified cloud providers. The list is iterated in order and the first provider that succeeds is used. Provider-specific data is written under `/run/config`; user data is stored at `path`.

```yaml
stages:
  default:
    - name: "Fetch cloud user data"
      datasource:
        providers:
          - "aws"
          - "digitalocean"
        path: "/etc/cloud-data"
```

### Commands

#### `stages.<stageID>.[<stepN>].commands`

Runs arbitrary shell commands. Executes after file/directory writes so newly written files and directories are available.

```yaml
stages:
  default:
    - name: "Regenerate initrd"
      commands:
        - dracut -f
        - depmod -a
```
