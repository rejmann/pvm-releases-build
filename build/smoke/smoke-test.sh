#!/bin/sh
# Runs inside each distro container from build.yml's smoke-test matrix.
# Extracts the tarball under test (bind-mounted at /php.tar.gz), then
# exercises exactly what CLAUDE.md's own manual Docker testing checked:
# `php -v`, `php -m`, and a script touching curl/openssl/zip/json so a
# statically-missing symbol fails loudly instead of only at runtime for a
# real user.
set -eu

expected_version="${1:?usage: smoke-test.sh <expected-php-version>}"

mkdir -p /opt/php
tar -xzf /php.tar.gz -C /opt/php --strip-components=1
php_bin=/opt/php/bin/php
if [ ! -x "$php_bin" ]; then
  echo "smoke-test: no executable bin/php after extracting the tarball" >&2
  find /opt/php -maxdepth 2 >&2
  exit 1
fi

# The tarball ships no php.ini: pvm generates one at install time pointing
# openssl.cafile/curl.cainfo at the distro's CA bundle and passes it via
# PHPRC, with PHP_INI_SCAN_DIR isolating it from any system PHP config
# (CLAUDE.md section 7). Do the same here so the smoke test exercises the
# binary the way pvm will actually run it.
ca_bundle=""
for f in \
  /etc/ssl/certs/ca-certificates.crt \
  /etc/pki/tls/certs/ca-bundle.crt \
  /etc/ssl/ca-bundle.pem \
  /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem \
  /etc/ssl/cert.pem; do
  if [ -r "$f" ]; then
    ca_bundle="$f"
    break
  fi
done
if [ -z "$ca_bundle" ]; then
  echo "smoke-test: no CA bundle found in any known location" >&2
  exit 1
fi
mkdir -p /opt/php/etc/conf.d
cat > /opt/php/etc/php.ini <<EOF
openssl.cafile=$ca_bundle
curl.cainfo=$ca_bundle
EOF
export PHPRC=/opt/php/etc
export PHP_INI_SCAN_DIR=/opt/php/etc/conf.d

echo "== php -v =="
actual_version="$("$php_bin" -v | head -1)"
echo "$actual_version"
case "$actual_version" in
  *"PHP $expected_version"*) ;;
  *)
    echo "smoke-test: expected PHP $expected_version, got: $actual_version" >&2
    exit 1
    ;;
esac

echo "== php -m =="
"$php_bin" -m

echo "== extension smoke script =="
"$php_bin" -r '
$fns = ["curl_init", "openssl_encrypt", "json_encode", "mb_strtoupper", "zip_open"];
foreach ($fns as $fn) {
    if (!function_exists($fn)) {
        fwrite(STDERR, "missing function: $fn\n");
        exit(1);
    }
}
$cafile = ini_get("openssl.cafile");
if ($cafile === false || $cafile === "" || !is_readable($cafile)) {
    fwrite(STDERR, "openssl.cafile not set to a readable file: " . var_export($cafile, true) . "\n");
    exit(1);
}
echo "ok: curl/openssl/json/mbstring/zip present, openssl.cafile=$cafile\n";
'

echo "smoke-test: PASS"
