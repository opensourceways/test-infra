FROM swr.ap-southeast-1.myhuaweicloud.com/opensourceway/prow/bazel2-env:latest as builder
LABEL maintainer="Xie Weizhi" description="The prow hookagent module image"
WORKDIR test-infra
COPY . .
RUN bazel build prow/external-plugins/hookagent

FROM centos:latest
WORKDIR /root/
COPY --from=builder /usr/local/test-infra/bazel-bin/prow/external-plugins/hookagent/linux_amd64_pure_stripped/hookagent .
COPY --from=builder /usr/local/test-infra/prow/external-plugins/hookagent/depoly/depoly.sh .
RUN chmod +x depoly.sh && ./depoly.sh && ls
ENTRYPOINT ["./hookagent"]
EXPOSE 8080