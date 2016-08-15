function mount_btrfs {
  sudo mount -tcgroup -odevices cgroup:devices /sys/fs/cgroup
  devices_mount_info=$(cat /proc/self/cgroup | grep devices)
  devices_subdir=$(echo $devices_mount_info | cut -d: -f3)
  sudo su -c "echo 'b 7:* rwm' > /sys/fs/cgroup/devices.allow"
  sudo su -c "echo 'b 7:* rwm' > /sys/fs/cgroup${devices_subdir}/devices.allow"
  for i in {0..256}
  do
    sudo mknod -m777 /dev/loop$i b 7 $i
  done
  sudo mkdir /mnt/btrfs
  sudo mount -t btrfs -o user_subvol_rm_allowed,rw /btrfs_volume /mnt/btrfs
  sudo chmod 777 -R /mnt/btrfs
  sudo btrfs quota enable /mnt/btrfs
  sudo mkdir -p /mnt/btrfs/bundles
  sudo chown 1000:1000 /mnt/btrfs/bundles
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
