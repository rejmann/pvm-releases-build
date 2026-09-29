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
