# AR Drive API (OpenAPI 3.1)

```yaml
openapi: 3.1.0
info:
  title: AR Drive API
  version: 0.1.0
  description: |
    Proxy API над Cloud.ru Artifact Registry. Все ручки (кроме `/healthz`,
    `/readyz`, `/metrics`, `/s/{shortID}`) требуют заголовки:
      X-Cloudru-Client-Id, X-Cloudru-Client-Secret, X-Cloudru-Project-Id
servers:
  - url: http://localhost:8080

components:
  securitySchemes:
    CloudRuKeys:
      type: apiKey
      in: header
      name: X-Cloudru-Client-Id
  schemas:
    Error:
      type: object
      properties:
        error: { type: string }
        code:  { type: string }
    Registry:
      type: object
      properties:
        id:           { type: string, format: uuid }
        name:         { type: string }
        projectId:    { type: string, format: uuid }
        registryType: { type: string, enum: [DOCKER, DEBIAN, RPM, GENERIC] }
        status:       { type: string, enum: [CREATING, ACTIVE, ERROR] }
        isPublic:     { type: boolean }
        tariff:       { type: string, enum: [BASIC, PREMIUM] }
        createdAt:    { type: string, format: date-time }
        updatedAt:    { type: string, format: date-time }
    File:
      type: object
      properties:
        path:        { type: string }
        name:        { type: string }
        size:        { type: integer, format: int64 }
        sha256:      { type: string }
        contentType: { type: string }
        updatedAt:   { type: string, format: date-time }
    Folder:
      type: object
      properties:
        path:    { type: string }
        virtual: { type: boolean }
    Share:
      type: object
      properties:
        short_id:      { type: string }
        url:           { type: string, format: uri }
        project_id:    { type: string }
        registry_id:   { type: string }
        file_path:     { type: string }
        has_password:  { type: boolean }
        max_downloads: { type: integer }
        downloads:     { type: integer }
        expires_at:    { type: string, format: date-time }
        revoked:       { type: boolean }
        created_at:    { type: string, format: date-time }

paths:
  /healthz:
    get:
      summary: Liveness probe
      responses: { '200': { description: ok } }
  /readyz:
    get:
      summary: Readiness probe
      responses: { '200': { description: ready } }
  /metrics:
    get:
      summary: Prometheus metrics
      responses: { '200': { description: prometheus text exposition } }

  /api/v1/auth/verify:
    post:
      summary: Проверить и закешировать креды Cloud.ru
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [client_id, client_secret, project_id]
              properties:
                client_id:     { type: string }
                client_secret: { type: string }
                project_id:    { type: string, format: uuid }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  ok:        { type: boolean }
                  user_key:  { type: string }
        '401': { description: Invalid credentials, content: { application/json: { schema: { $ref: '#/components/schemas/Error' } } } }
        '502': { description: IAM unavailable }

  /api/v1/registries:
    get:
      summary: Список реестров проекта
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: query
          name: pageSize
          schema: { type: integer, default: 50 }
        - in: query
          name: pageToken
          schema: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:         { type: array, items: { $ref: '#/components/schemas/Registry' } }
                  nextPageToken: { type: string }

  /api/v1/registries/{registryID}/files:
    get:
      summary: Список файлов и подпапок текущей директории
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: path
          name: registryID
          required: true
          schema: { type: string }
        - in: query
          name: path
          schema: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  files:   { type: array, items: { $ref: '#/components/schemas/File' } }
                  folders: { type: array, items: { $ref: '#/components/schemas/Folder' } }
                  nextPageToken: { type: string }
    post:
      summary: Загрузить файл (multipart)
      security: [ { CloudRuKeys: [] } ]
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
                path: { type: string, description: 'Целевая папка (например "dir/sub/")' }
      responses:
        '201': { description: Created, content: { application/json: { schema: { $ref: '#/components/schemas/File' } } } }
        '413': { description: File too large }
    delete:
      summary: Удалить файл (path в query)
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: query
          name: path
          required: true
          schema: { type: string }
      responses:
        '204': { description: deleted }

  /api/v1/registries/{registryID}/files/download:
    get:
      summary: Скачать файл
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: query
          name: path
          required: true
          schema: { type: string }
      responses:
        '200': { description: octet stream }

  /api/v1/registries/{registryID}/folders:
    post:
      summary: Создать виртуальную папку (маркер)
      security: [ { CloudRuKeys: [] } ]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [path]
              properties:
                path: { type: string }
      responses:
        '201': { description: Created }
        '409': { description: Already exists }
    delete:
      summary: Удалить папку (со всеми файлами под префиксом)
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: query
          name: path
          required: true
          schema: { type: string }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  deleted_files: { type: integer }
                  path:          { type: string }

  /api/v1/shares:
    get:
      summary: Список созданных пользователем ссылок
      security: [ { CloudRuKeys: [] } ]
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  items: { type: array, items: { $ref: '#/components/schemas/Share' } }
    post:
      summary: Создать шер-ссылку
      security: [ { CloudRuKeys: [] } ]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [registry_id, file_path]
              properties:
                registry_id:   { type: string }
                file_path:     { type: string }
                password:      { type: string }
                ttl_seconds:   { type: integer }
                max_downloads: { type: integer }
      responses:
        '201': { description: Created, content: { application/json: { schema: { $ref: '#/components/schemas/Share' } } } }

  /api/v1/shares/{shortID}:
    delete:
      summary: Отозвать ссылку
      security: [ { CloudRuKeys: [] } ]
      parameters:
        - in: path
          name: shortID
          required: true
          schema: { type: string }
      responses:
        '204': { description: Revoked }
        '404': { description: Not found }

  /s/{shortID}:
    get:
      summary: Публичное скачивание по шер-ссылке
      parameters:
        - in: path
          name: shortID
          required: true
          schema: { type: string }
        - in: query
          name: password
          schema: { type: string }
      responses:
        '200': { description: octet stream }
        '401': { description: password required / wrong }
        '410': { description: revoked / expired / exhausted }
```
