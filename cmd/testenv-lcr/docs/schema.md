# testenv-lcr Configuration



> Full OpenAPI specification: [spec.openapi.yaml](../spec.openapi.yaml)

## Fields

### `enabled`

- **Type:** `boolean`
- **Required:** No
- **Description:** Whether the local container registry is enabled

### `imagePullSecretName`

- **Type:** `string`
- **Required:** No
- **Description:** Name of the image pull secret to create

### `imagePullSecretNamespaces`

- **Type:** `array of string`
- **Required:** No
- **Description:** List of namespaces to create image pull secrets in

### `images`

- **Type:** `array of `
- **Required:** No
- **Description:** Images to push into the registry once it is up. A local:// name is an image the build stage produced on this machine; anything else is pulled from its registry, with basicAuth when it is private.

### `namespace`

- **Type:** `string`
- **Required:** No
- **Description:** Kubernetes namespace for the registry deployment

