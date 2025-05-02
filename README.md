# auth-alert
Istio AuthorizationPolicy 변경 감시 및 Slack 알림 컨트롤러

## 설명
이 컨트롤러는 Istio의 AuthorizationPolicy 리소스를 모니터링하여
remoteIpBlocks 또는 hosts 필드에 변경(추가)이 발생할 때마다
설정된 Slack 채널로 알림을 전송합니다.

## Prerequisites
- Go v1.22+
- Docker 17.03+
- kubectl v1.11.3+
- Istio authorizationpolicies CRD가 설치된 Kubernetes 클러스터

## 설치 및 배포

1. 이미지 빌드·푸시
```sh
make docker-build IMG=<registry>/auth-alert:<version>
make docker-push  IMG=<registry>/auth-alert:<version>
```

2. Slack Webhook 설정
`config/manager/manager.yaml`의 `containers` 아래 `env` 섹션에 다음을 추가합니다.
```yaml
- name: SLACK_WEBHOOK_URL
  value: "<https://hooks.slack.com/services/XXX/YYY/ZZZ>"
```

3. 배포
```sh
make deploy IMG=<registry>/auth-alert:<version>
```

4. 실행 확인
```sh
kubectl -n auth-alert-system get deploy
kubectl -n auth-alert-system logs deploy/auth-alert-controller-manager -c manager
```

## 삭제
```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/auth-alert:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/auth-alert/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

