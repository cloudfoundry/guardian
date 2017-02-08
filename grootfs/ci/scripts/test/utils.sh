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

  # Make BTRFS volume
  truncate -s 1G /btrfs_volume
  mkfs.btrfs /btrfs_volume

  # Mount BTRFS
  mkdir /mnt/btrfs
  mount -t btrfs -o user_subvol_rm_allowed,rw /btrfs_volume /mnt/btrfs
  chmod 777 -R /mnt/btrfs
  btrfs quota enable /mnt/btrfs

  # Make XFS Volume
  truncate -s 1G /xfs_volume_1
  mkfs.xfs /xfs_volume_1
  truncate -s 1G /xfs_volume_2
  mkfs.xfs /xfs_volume_2
  truncate -s 1G /xfs_volume_3
  mkfs.xfs /xfs_volume_3
  truncate -s 1G /xfs_volume_4
  mkfs.xfs /xfs_volume_4

  # Mount XFS
  mkdir /mnt/xfs-1
  mount -t xfs -o pquota /xfs_volume_1 /mnt/xfs-1
  chmod 777 -R /mnt/xfs-1

  mkdir /mnt/xfs-2
  mount -t xfs -o pquota /xfs_volume_2 /mnt/xfs-2
  chmod 777 -R /mnt/xfs-2

  mkdir /mnt/xfs-3
  mount -t xfs -o pquota /xfs_volume_3 /mnt/xfs-3
  chmod 777 -R /mnt/xfs-3

  mkdir /mnt/xfs-4
  mount -t xfs -o pquota /xfs_volume_4 /mnt/xfs-4
  chmod 777 -R /mnt/xfs-4
}

sudo_mount_storage() {
  local MOUNT_STORAGE_FUNC=$(declare -f mount_storage)
  sudo bash -c "$MOUNT_STORAGE_FUNC; mount_storage"
}

move_to_gopath() {
  thing_i_want_moved=$1
  dest_path=/go/src/code.cloudfoundry.org/${thing_i_want_moved}

  # remove the original grootfs package path
  [ -d $dest_path ] && rmdir $dest_path

  # link the uploaded source (from build) to the GOPATH
  ln -s $PWD/src/code.cloudfoundry.org/${thing_i_want_moved} $dest_path

  # because the uploaded source is owned by the user that runs fly, we need
  # to chown
  pushd $dest_path
    sudo chown -R groot:groot .
  popd

  echo $dest_path
}

install_dependencies() {
  if ! [ -d vendor ]; then
    glide install
  fi
}

setup_drax() {
  drax_path=$1
  cp $drax_path /usr/local/bin/drax
  chown root:root /usr/local/bin/drax
  chmod u+s /usr/local/bin/drax
}

sudo_setup_drax() {
  drax_path=$1

  local SETUP_DRAX_FUNC=$(declare -f setup_drax)
  sudo bash -c "$SETUP_DRAX_FUNC; setup_drax $drax_path"
}
