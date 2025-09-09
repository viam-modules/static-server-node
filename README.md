# Module static-server-node

Provide a description of the purpose of the module and any relevant information.

## Model viam:static-server-node:server

Provide a description of the model and any relevant information.

### Configuration

The following attribute template can be used to configure this model:

```json
{
"attribute_1": <float>,
"attribute_2": <string>
}
```

#### Attributes

The following attributes are available for this model:

| Name            | Type   | Inclusion | Description                                                                                                                                                                                                                                      |
| --------------- | ------ | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `path`          | string | Required  | The path to the web project. Currently only supports Github URLS in the format `git+https://github.com/OWNER/REPO#REF`, where `OWNER` and `REPO` are required, and `#REF` is an optional string pointing to a branch/commit (defaults to `main`) |
| `access_token`  | string | Optional  | If your Github repository is private, provide the Access Token                                                                                                                                                                                   |
| `node_version`  | string | Optional  | The node version to install. Defaults to latest LTS, which is currently `22.19.0`                                                                                                                                                                |
| `build_command` | string | Optional  | Specific npm command to run (defaults to `build`). `npm run` is automatically prepended, so the final command is `npm run BIULD_COMMAND`.                                                                                                        |
| `build_dir`     | string | Optional  | The output directory of the `build_command` (defaults to `dist`)                                                                                                                                                                                 |
| `port`          | int    | Optional  | Port to serve on (defaults to `8888`)                                                                                                                                                                                                            |

#### Example Configuration

```json
{
  "path": "git+https://github.com/njooma/my-svelte-app#release",
  "access_token": "gpg_MY_ACCESS_TOKEN",
  "build_command": "specific_build"
}
```
