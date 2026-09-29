# Parametrized smoke-test image: one Dockerfile, one FROM per distro in the
# matrix build.yml drives, matching the table validated in CLAUDE.md
# (~/Projects/pvm) section 3. Build with --build-arg BASE_IMAGE=<image>.
#
# Only debian/ubuntu/fedora/arch/opensuse/rockylinux need ca-certificates
# installed explicitly for the static PHP binary's OpenSSL to find
# /etc/ssl/cert.pem (CLAUDE.md section 7) — alpine ships it by default.
ARG BASE_IMAGE=debian:12-slim
FROM ${BASE_IMAGE}

COPY smoke-test.sh /smoke-test.sh
COPY install-ca-certs.sh /install-ca-certs.sh
RUN chmod +x /smoke-test.sh /install-ca-certs.sh && /install-ca-certs.sh

# The tarball under test is bind-mounted at /php.tar.gz by build.yml.
ENTRYPOINT ["/smoke-test.sh"]
