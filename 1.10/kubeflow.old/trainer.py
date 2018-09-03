import sys
import os
import ast

from tensorflow.examples.tutorials.mnist import input_data
mnist = input_data.read_data_sets("MNIST_data/", one_hot=True)

import tensorflow as tf

FLAGS = None


def main(_):
  # Create a cluster from the parameter server and worker hosts.
  #cluster = tf.train.ClusterSpec({"ps": FLAGS['ps_hosts'], "worker": FLAGS['worker_hosts'], "eval": FLAGS['eval_hosts']})
  cluster = tf.train.ClusterSpec(TF_CONFIG["cluster"])

  # Create and start a server for the local task.
  server = tf.train.Server(cluster,
                           job_name=FLAGS['job_name'],
                           task_index=FLAGS['task_index'])

  if FLAGS['job_name'] == "ps":
    server.join()
  elif FLAGS['job_name'] == "worker":

    # Assigns ops to the local worker by default.
    with tf.device(tf.train.replica_device_setter(
        worker_device="/job:worker/task:%d" % FLAGS['task_index'],
        cluster=cluster)):

      # Build model...
      x = tf.placeholder(tf.float32, [None, 784])
      W = tf.Variable(tf.zeros([784, 10]))
      b = tf.Variable(tf.zeros([10]))
      y = tf.nn.softmax(tf.matmul(x, W) + b)
      y_ = tf.placeholder(tf.float32, [None, 10])
      cross_entropy = tf.reduce_mean(-tf.reduce_sum(y_ * tf.log(y), reduction_indices=[1]))
      learning_rate = 0.05
      global_step = tf.train.get_or_create_global_step()
      train_step = tf.train.GradientDescentOptimizer(learning_rate).minimize(cross_entropy, global_step=global_step)

    # The StopAtStepHook handles stopping after running given steps.
    hooks=[tf.train.StopAtStepHook(last_step=100000)]

    # The MonitoredTrainingSession takes care of session initialization,
    # restoring from a checkpoint, saving to a checkpoint, and closing when done
    # or an error occurs.
    with tf.train.MonitoredTrainingSession(master=server.target,
                                           is_chief=(FLAGS['task_index'] == 0),
                                           summary_dir="/tmp/train_logs",
                                           hooks=hooks) as mon_sess:

      steppp=0
      while not mon_sess.should_stop():
        #for _ in range(1000):
        batch_xs, batch_ys = mnist.train.next_batch(100)
        mon_sess.run(train_step, feed_dict={x: batch_xs, y_: batch_ys})
        steppp = steppp + 1
        #break
      sys.stderr.write('global_step: '+str(steppp)+'\n')


if __name__ == "__main__":
  TF_CONFIG_str = os.environ["TF_CONFIG"]
  TF_CONFIG = ast.literal_eval(TF_CONFIG_str)
  FLAGS = {'job_name': TF_CONFIG["task"]["type"], 'task_index': TF_CONFIG["task"]["index"]}
  tf.app.run(main=main, argv=[sys.argv[0]])
