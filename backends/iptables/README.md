# Naive impl 

1) Write iptables rules manually

2) Pipe to iptables restore

# Legacy impl

1) import datastructures from config.go that are used to trigger watches

2) import proxier.go, which writes iptables rules periodically, and implements the interfaces in (1)

3) use KPNG to replace the code in server.go

# Correct implementation

1) 

2) 

3) 