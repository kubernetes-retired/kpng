# What is a job ?

KPNG converts a number of input sources into backend destinations, and because of this, there need to be a number of separate jobs which KPNG runs depending on user intent.  

Typically a user defines:
-  a source (i.e. kube)
- a sink (i.e. local)

Then, the KPNG server (or "spine") connects these inputs and outputs.  The process which runs this connection is a "job".

For example, the "kube2store" job runs a process which continually watches the 
ubernetes APIServer.  
The "store2api" job watches the in memory store of the kpng network model and publishes it as an api for others to consume, and so on. 

These jobs are invoked from the wrapper programs in the cmd/kpng/ package, for example k2s, f2s, and so on.

