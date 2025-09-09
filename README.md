# Module static-server-node

A module to build and serve static HTML, building with node first.

## Model viam:static-server-node:server

The static server. Upon creation, this server will

1. Download node (or use already downloaded version)
1. Locate the project directory, downloading if remote
   - Currently, only remote Github repositories are supported
1. Build the project using the `build_command` attribute in the config (defaulting to `npm run build`)
1. Serve the built output from the `build_dir` directory (defaulting to `./dist`) at the specified `port` (defaulting to `8888`)

### Configuration

The following attribute template can be used to configure this model:

```json
{
  "path": <string>,
  "access_token": <string>,
  "node_version": <string>,
  "build_command": <string>,
  "build_dir": <string>,
  "port": <int>
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
