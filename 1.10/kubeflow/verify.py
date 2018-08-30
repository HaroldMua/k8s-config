import sys
import os
import ast

from tensorflow.examples.tutorials.mnist import input_data
mnist = input_data.read_data_sets("MNIST_data/", one_hot=True)

import tensorflow as tf

FLAGS = None
TF_CONFIG = None

def main(_):
  # Create a cluster from the parameter server and worker hosts.
  #cluster = tf.train.ClusterSpec({"ps": FLAGS['ps_hosts'], "worker": FLAGS['worker_hosts'], "eval": FLAGS['eval_hosts']})
  cluster = tf.train.ClusterSpec(TF_CONFIG["cluster"])

  # Create and start a server for the local task.
  server = tf.train.Server(cluster,
                           job_name=FLAGS['job_name'],
                           task_index=FLAGS['task_index'])

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
    #init_op = tf.global_variables_initializer()

    correct_prediction = tf.equal(tf.argmax(y,1), tf.argmax(y_,1))
    accuracy = tf.reduce_mean(tf.cast(correct_prediction, tf.float32))

  with tf.Session(server.target) as sess:
    #sess.run(init_op)
    print(sess.run(accuracy, feed_dict={x: mnist.test.images, y_: mnist.test.labels}))



if __name__ == "__main__":
  TF_CONFIG_str = os.environ["TF_CONFIG"]
  TF_CONFIG = ast.literal_eval(TF_CONFIG_str)
  FLAGS = {'job_name': TF_CONFIG["task"]["type"], 'task_index': TF_CONFIG["task"]["index"]}
  tf.app.run(main=main, argv=[sys.argv[0]])
