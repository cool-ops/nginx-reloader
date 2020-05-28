# nginx-reloader

用于监听Nginx配置变化，自动reload Nginx  

示例：
```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      name: nginx
      labels:
        app: nginx
    spec:
      shareProcessNamespace: true
      containers:
        - name: nginx
          image: nginx
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: nginx-config
              mountPath: /etc/nginx/conf.d
              readOnly: true           
        - name: nginx-reloader
          image: registry.cn-hangzhou.aliyuncs.com/rookieops/nginx-reloader:v2
          imagePullPolicy: IfNotPresent
          env:
            - name: WATCH_NGINX_CONF_PATH
              value: /etc/nginx
          volumeMounts:
          - name: nginx-config
            mountPath: /etc/nginx/conf.d
            readOnly: true
      volumes:
      - name: nginx-config
        configMap:
          name: nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
data:
  default.conf: |-
      server {
        server_name localhost;
        listen 80 default_server;

        location = /healthz {
          add_header Content-Type text/plain;
          return 200 'ok';
        }

        location / {
            root   /usr/share/nginx/html;
            index  index.html index.htm;
        }

        error_page   500 502 503 504  /50x.html;

        location = /50x.html {
            root   /usr/share/nginx/html;
        }
      }

```