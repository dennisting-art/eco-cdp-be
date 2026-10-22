package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed swagger-ui/index.html
var SwaggerUI []byte
