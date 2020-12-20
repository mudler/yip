# :pushpin: yip


Simply applies a configuration to the system described with yaml files.


```yaml
stages:
   # "test" is the stage
   test:
     - files:
        - path: /tmp/foo
          content: |
                    test
          permissions: 0777
          owner: 1000
          group: 100
       commands:
        - echo "test"
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

```bash
$> yip -s test yip1.yaml yip2.yaml
$> yip -s test https://..
```

Yip uses a simple, yet powerful distro-agnostic cloud-init style format for the definition.

## How it works


Yip works in stages. You can define "stages" that you can apply in various ways and in a different enviroment (that's why *stages*).  
Yip also support setting up system dns and integrates accounting support with [entities](https://github.com/mudler/entities)

A stage is just a list of commands and files to write in the system, for example:

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

Now we can execute it:

```bash
$> cat myfile.yaml | yip -
```

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


That's it! by default `yip` uses the default stage and the `default` executor, but you can customize its execution.


```
yip loads cloud-init style yamls and applies them in the system.

For example:

        $> yip -s initramfs https://<yip.yaml> <definition.yaml> ...
        $> yip -s initramfs <yip.yaml> <yip2.yaml> ...
        $> cat def.yaml | yip -

Usage:
  yip [flags]

Flags:
  -e, --executor string   Executor which applies the config (default "default")
  -h, --help              help for yip
  -s, --stage string      Stage to apply (default "default")
```
