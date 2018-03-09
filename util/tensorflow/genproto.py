import os
import subprocess


def list_files(prefix, src_dir, suffix):
  files = os.listdir(prefix+"/"+src_dir)
  protos = []
  for f in files:
    if not f.endswith(suffix):
      continue
    protos.append(src_dir+"/"+f)
  return protos


def new_go_out_flag(go_path, dst_pkg, src_list):
  flags = []

  import_path = dst_pkg.split("/")[-1]
  flags.append("import_path={}".format(import_path))

  for src in src_list:
    mod = "M{}={}".format(src, dst_pkg)
    flags.append(mod)

  flag_str = "--go_out=" 
  flag_str += ",".join(flags)
  flag_str += ":{}/src/{}".format(go_path, dst_pkg)
  return flag_str


# Specify the protoc plugins.
go_path = os.environ["GOPATH"]
plugin_flag = "--plugin={}/bin/protoc-gen-go".format(go_path)

# Specify the arguments to the protobuf Go plugin.
dst_pkg = "github.com/fumin/bangbang/util/tensorflow/protos_all_go_proto"
go_tf_pkg = "github.com/tensorflow/tensorflow"
prefix = go_path + "/src/" + go_tf_pkg
proto_pkg_list = []
proto_pkg_list.append("tensorflow/core/framework")
proto_pkg_list.append("tensorflow/core/protobuf")
proto_pkg_list.append("tensorflow/core/lib/core")
src_list = []
for ppkg in proto_pkg_list:
  src_list += list_files(prefix, ppkg, ".proto")
go_out_flag = new_go_out_flag(go_path, dst_pkg, src_list)

# Specify the proto_paths.
proto_path_flags = []
protobuf_archive = "bazel-tensorflow/external/protobuf_archive"
proto_proto_path = "{}/src/{}/{}/src".format(go_path, go_tf_pkg, protobuf_archive)
proto_path_flags.append("--proto_path={}".format(proto_proto_path))
proto_path_flags.append("--proto_path={}".format(prefix))

# Specify the protos to compile.
in_file_list = []
for ppkg in proto_pkg_list:
  src_list += list_files(prefix, ppkg, ".proto")
  for src in src_list:
    fstr = "{}/{}".format(prefix, src)
    in_file_list.append(fstr)

# Generate all Go protos.
protoc = "{}/src/{}/{}/bazel-bin/protoc".format(go_path, go_tf_pkg, protobuf_archive)
protoc_args = [plugin_flag, go_out_flag] + proto_path_flags + in_file_list
subprocess.call([protoc] + protoc_args)

# Move all generated Go protos to dst_pkg
dst_dir = "{}/src/{}".format(go_path, dst_pkg)
for ppkg in proto_pkg_list:
  src_list = list_files(dst_dir, ppkg, ".pb.go")
  for src in src_list:
    subprocess.call(["mv", "{}/{}".format(dst_dir, src), dst_dir])
subprocess.call(["rm", "-r", "{}/tensorflow".format(dst_dir)])
