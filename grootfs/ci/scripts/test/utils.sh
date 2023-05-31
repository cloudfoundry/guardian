mount_storage() {
  # Configure cgroup
  mount -t tmpfs cgroup_root /sys/fs/cgroup
  mkdir -p /sys/fs/cgroup/devices
  mkdir -p /sys/fs/cgroup/memory

  mount -tcgroup -odevices cgroup:devices /sys/fs/cgroup/devices
  devices_mount_info=$(cat /proc/self/cgroup | grep devices)
  devices_subdir=$(echo $devices_mount_info | cut -d: -f3)
  echo 'b 7:* rwm' > /sys/fs/cgroup/devices/devices.allow
  echo 'b 7:* rwm' > /sys/fs/cgroup/devices${devices_subdir}/devices.allow

  mount -tcgroup -omemory cgroup:memory /sys/fs/cgroup/memory

  # Setup loop devices
  for i in {0..256}
  do
    mknod -m777 /dev/loop$i b 7 $i
  done

  # Make and Mount EXT4 Volume
  mkdir /mnt/ext4
  truncate -s 256M /ext4_volume
  mkfs.ext4 /ext4_volume
  mount /ext4_volume /mnt/ext4
  chmod 777 /mnt/ext4

  for i in {1..10}
  do
    # Make XFS Volume
    truncate -s 3G /xfs_volume_${i}
    mkfs.xfs -b size=4096 /xfs_volume_${i}

    # Mount XFS
    mkdir /mnt/xfs-${i}
    if ! mount -t xfs -o pquota,noatime /xfs_volume_${i} /mnt/xfs-${i}; then
      free -h
      echo Mounting xfs failed, bailing out early!
      echo NOTE: this might be because of low system memory, please check out output from free above
      exit 13
    fi
    chmod 777 -R /mnt/xfs-${i}
  done
}

unmount_storage() {
  umount -l /mnt/ext4

  for i in {1..10}
  do
    umount -l /mnt/xfs-${i}
    rmdir /mnt/xfs-${i}
    rm /xfs_volume_${i}
  done

  rmdir /mnt/ext4
  rm /ext4_volume

  for i in {0..256}; do
    rm /dev/loop$i
  done
}

sudo_mount_storage() {
  local MOUNT_STORAGE_FUNC=$(declare -f mount_storage)
  sudo bash -c "$MOUNT_STORAGE_FUNC; mount_storage"
}

sudo_unmount_storage() {
  local UNMOUNT_STORAGE_FUNC=$(declare -f unmount_storage)
  sudo bash -c "$UNMOUNT_STORAGE_FUNC; unmount_storage"
}

install_dependencies() {
  if ! [ -d vendor ]; then
    glide install
  fi
}

