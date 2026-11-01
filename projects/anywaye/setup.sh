#!/usr/bin/env bash
# Anywaye project setup — runs on the dev VPS after base provisioning.
set -euo pipefail

BASE_DIR="${BASE_DIR:-/code}"
CODE_DIR="$BASE_DIR/anywaye"

echo "=== Setting up anywaye project ==="

mkdir -p "$CODE_DIR"
cd "$CODE_DIR"

REPOS="ship website identity gateway ella billing courier core darkroom keyholder kit loom notes oauth-stub ops platform press anywaye-py anywaye-rb anywaye-ts"
for repo in $REPOS; do
  if [ -d "$repo/.git" ]; then
    echo "$repo: pulling..."
    git -C "$repo" pull --ff-only 2>&1 | tail -1
  else
    echo "$repo: cloning..."
    git clone "https://$GH_TOKEN@github.com/cafaye/$repo.git" "$repo" 2>&1 | tail -1 || echo "$repo: clone failed"
  fi
done

echo ""
echo "Shared infra setup deferred. Run:"
echo "  cd $CODE_DIR/identity"
echo "  export KIT_STACK_DIR=$CODE_DIR/kit"
echo "  export KIT_STACK_REF=e20d20664e6f1dc0bfb1023874367d2be43370d5"
echo "  export KIT_DEV_PROFILES=''  # skip observability (prod-only)"
echo "  export KIT_POSTGRES_DATABASES='identity,ship,ella,cafe,core,courier,billing,gateway,keyholder,kit,loom,notes,oauth-stub,platform,press,website,docs,anywaye-py,anywaye-rb,anywaye-ts,box,darkroom'"
echo "  export COURIER_TOKEN=dummy"
echo "  ./bin/dev"

echo ""
echo "Anywave project set up in $CODE_DIR"