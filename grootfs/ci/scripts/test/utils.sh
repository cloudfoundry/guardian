function mount_btrfs {
  # Configure cgroup
  mount -tcgroup -odevices cgroup:devices /sys/fs/cgroup
  devices_mount_info=$(cat /proc/self/cgroup | grep devices)
  devices_subdir=$(echo $devices_mount_info | cut -d: -f3)
  echo 'b 7:* rwm' > /sys/fs/cgroup/devices.allow
  echo 'b 7:* rwm' > /sys/fs/cgroup${devices_subdir}/devices.allow

  # Setup loop devices
  for i in {0..256}
  do
    mknod -m777 /dev/loop$i b 7 $i
  done

  # Make BTRFS volume
  truncate -s 1G /btrfs_volume
  mkfs.btrfs /btrfs_volume

  # Mount BTRFS
  mkdir /mnt/btrfs
  mount -t btrfs -o user_subvol_rm_allowed,rw /btrfs_volume /mnt/btrfs
  chmod 777 -R /mnt/btrfs
  btrfs quota enable /mnt/btrfs
}

function sudo_mount_btrfs {
  local MOUNT_BTRFS_FUNC=$(declare -f mount_btrfs)
  sudo bash -c "$MOUNT_BTRFS_FUNC; mount_btrfs"
}

function show_groot_banner {
  cat $(dirname $BASH_SOURCE)/groot.ascii
  echo
  echo "I AM $(whoami)"
  echo
}

function move_to_gopath {
  grootfsPath=/go/src/code.cloudfoundry.org/grootfs
  rmdir $grootfsPath
  ln -s $PWD/src/code.cloudfoundry.org/grootfs $grootfsPath
  cd $grootfsPath
}
