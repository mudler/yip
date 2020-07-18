# :pushpin: yip

Simply applies a configuration to the system described with yaml files.


```yaml
stages:
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
```
$> yip -s test <entity.yaml>

- Simple
- Small scope, pluggable, extensible
