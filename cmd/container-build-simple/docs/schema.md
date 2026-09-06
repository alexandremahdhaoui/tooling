# container-build-simple Configuration

Assemble a container image from prebuilt files and a base, with no daemon, no Containerfile and no shellout

> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `base`

- **Type:** `string`
- **Required:** No
- **Description:** The base image. Use "scratch" for none.
A static binary runs on scratch, so the base exists for whatever else runs alongside it. For a CI job container that is a shell and glibc, which is why the default is a debian and not an alpine: musl breaks JavaScript actions in edge cases, and every "uses:" step in a workflow is a JavaScript action.


### `binDir`

- **Type:** `string`
- **Required:** No
- **Description:** Where the files land inside the image, and the front of PATH.

### `env`

- **Type:** `map[string]string`
- **Required:** No
- **Description:** Environment variables to set in the image config.

### `from`

- **Type:** `array of string`
- **Required:** Yes
- **Description:** What goes into the layer, relative to the repository. Two shapes, told apart by what is on disk:
A directory holding a forge.yaml contributes that repository's built binaries by RECORD: every artifact of type binary its artifact store carries for one of the platforms this entry declares, each landing on its own platform under the artifact's name. Nothing is parsed out of a file name.
Anything else is a glob of files that land on every platform, because a script or a certificate is the same on all of them. A glob that matches nothing fails the build, because an image that silently ships empty fails on the runner that tries to use it, days later and far from the cause.
The platforms assembled are the build entry's own platforms: declaration, and each becomes one manifest in the index.


### `labels`

- **Type:** `map[string]string`
- **Required:** No
- **Description:** Labels to set in the image config.

