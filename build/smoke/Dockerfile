# Parametrized smoke-test image: one Dockerfile, one FROM per distro in the
# matrix build.yml drives, matching the table validated in CLAUDE.md
# (~/Projects/pvm) section 3. Build with --build-arg BASE_IMAGE=<image>.
#
# install-prereqs.sh installs tar (missing on the two minimal base images)
# and ca-certificates (so the static binary's OpenSSL finds a CA bundle —
# CLAUDE.md section 7); see that script for why neither is optional here.
ARG BASE_IMAGE=debian:12-slim
FROM ${BASE_IMAGE}

COPY smoke-test.sh /smoke-test.sh
COPY install-prereqs.sh /install-prereqs.sh
RUN chmod +x /smoke-test.sh /install-prereqs.sh && /install-prereqs.sh

# The tarball under test is bind-mounted at /php.tar.gz by build.yml.
ENTRYPOINT ["/smoke-test.sh"]
