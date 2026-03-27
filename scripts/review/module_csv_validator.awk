NR == 1 {
  if ($0 != expected) {
    printf "invalid module CSV header: %s\n", $0 > "/dev/stderr"
    exit 1
  }
  next
}

NF != 9 {
  printf "invalid module CSV row %d: expected 9 columns, got %d\n", NR, NF > "/dev/stderr"
  exit 1
}

($1 == "" || $2 == "" || $3 == "" || $4 == "" || $5 == "" || $6 == "" || $7 == "") {
  printf "invalid module CSV row %d: required columns 1-7 must be non-empty\n", NR > "/dev/stderr"
  exit 1
}

$1 in module_seen {
  printf "duplicate module_id on row %d: %s\n", NR, $1 > "/dev/stderr"
  exit 1
}

$5 in worktree_seen {
  printf "duplicate worktree_suffix on row %d: %s\n", NR, $5 > "/dev/stderr"
  exit 1
}

$6 in branch_seen {
  printf "duplicate branch_suffix on row %d: %s\n", NR, $6 > "/dev/stderr"
  exit 1
}

$1 !~ /^[A-Za-z0-9_]+$/ {
  printf "invalid module_id on row %d: %s\n", NR, $1 > "/dev/stderr"
  exit 1
}

$5 !~ /^[A-Za-z0-9._-]+$/ {
  printf "invalid worktree_suffix on row %d: %s\n", NR, $5 > "/dev/stderr"
  exit 1
}

$6 !~ /^[A-Za-z0-9._-]+$/ {
  printf "invalid branch_suffix on row %d: %s\n", NR, $6 > "/dev/stderr"
  exit 1
}

{
  module_seen[$1] = 1
  worktree_seen[$5] = 1
  branch_seen[$6] = 1
}

END {
  if (NR < 2) {
    print "module CSV has no module rows" > "/dev/stderr"
    exit 1
  }
}
