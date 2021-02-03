FROM swr.ap-southeast-1.myhuaweicloud.com/opensourceway/prow/bazel2-env:latest as builder
LABEL maintainer="Xie Weizhi" description="The prow module shares image"
WORKDIR test-infra
COPY . .
RUN bazel build prow/cmd/gitee-hook

FROM alpine:latest
RUN apk update && apk add git
WORKDIR /root/
COPY --from=builder /usr/local/test-infra/bazel-bin/prow/cmd/gitee-hook/linux_amd64_pure_stripped/gitee-hook ./app
ENTRYPOINT ["./app"]
EXPOSE ${BPORT}