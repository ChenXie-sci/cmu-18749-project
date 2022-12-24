## 18749 team-20 Project

### Language
* Server:   GoLanguage with version 1.18.2
* Client: Python with version 3.9
* LFD: GoLanguage with version 1.18.2
* GFD: ??


### Machines
* LFD/Server 1 -> 172.19.136.188 == ece017
* LFD/Server 2 -> 172.19.138.17 == ece015
* LFD/Server 3 -> 172.19.138.14 == ece012
* RM/GFD/Clients -> 172.19.136.190 == ece019

### Run 
* Server: 
    1.  `cd servers`
    2.  Run server  
        * `go run . server_name ip_address checkpointFreq`   
        * `go run . S1 172.19.136.188 10`    
        * `go run . S2 172.19.138.17 10`   
        * `go run . S3 172.19.138.14 10`   
* Client: 
    1.  `cd clients`
    2.  `python3 client.py -c config_path`   
        * `python3 M5_client.py -c c1.ini`    
        * `python3 M5_client.py -c c2.ini`    
        * `python3 M5_client.py -c c3.ini`    
* LFD: 
    1.  `cd lfd`
    2.  `go run . *heartbeat_freq* *lfd_name* *server_name* *server_port* *gfd_ip_port*`    
        * `go run . 10 LFD1 S1 172.19.136.189:8003 172.19.136.190:8000`   
        * `go run . 10 LFD2 S2 172.19.138.17:8003 172.19.136.190:8001`   
        * `go run . 10 LFD3 S3 172.19.138.14:8003 172.19.136.190:8002`
    * **Need to change path to the server's folder!!!!**   
* GFD: 
    1.  `cd gfd`
    2.  `go run . ip_address heartbeat_freq`\
        * `go run . 172.19.136.190 10`
* RM: 
    1.  `cd rm`
    2.  `go run . ip_address config`
        * `go run . 172.19.136.190 active`
        * `go run . 172.19.136.190 passive`
 
### Port usage
* Server 
  * 8000 for client1
  * 8001 for client2
  * 8002 for client3
  * 8003 for lfd
  * 8004 for replica1
  * 8005 for replica2
* GFD
  * 8000 for lfd1
  * 8001 for lfd2
  * 8002 for lfd3
* RM
  * 8003 for GFD

### IP usage
* Server 
  * "172.19.136.189" for server1
  * "172.19.138.17" for server2
  * "172.19.138.14" for server3
* GFD
  * "172.19.136.190" for gfd
* RM
  * "172.19.136.190" for RM

### Main Goal
#### Milestone A
* Server
  * Request & Response
    * Change status (string type)
  * Heartbeat response
  * Four ports
  * Print out the status before and after modifying the status
* Client
  * Send request to server and read response
  * Message should be based on the keyboard input or hard-coded (ask Priya)
  * Request number 
    * Increase by 1 when receive the request reponse from the server
  * Different request message every time (ex: "hello_world" + **count**)
  * Parameters
    * Server IP
    * Server Port
    * Server Name
    * Client Name
    * Time interval to print message
* LFD
  * Send the heartbeat with heart count
  * Changable heart frequency (not hard-coded)
  * Heartbeat frequency is the time between the previous sending and the next sending
  * Time-out server fail message 

#### Milestone B
* Server

* Client
  * Connect to three servers (Set config from each client)
  * Sent all message to three servers & print all messages
  * Print all response message from all three servers
  * Should ignore the fault server and continue to print message from other two servers
  * Detection of duplicate (?) and print (Req: 15)
  * Retry connection to server (milestone c)

* LFD
  * Reply hearbeat from GFD
  * Reply membership add/delete to GFD based on heartbeat from server side
  * Print membership
  * Retry connection to server (milestone c)

* GFD
  * member_count = 0 for initialization
  * print "GFD: 0 members"
  * Send hearbeat to LFD
  * Print add membership message, change membership arrray & memeber counts
  * Print delete membership message, change membership arrray & memeber counts

#### Milestone C
* Server
  * TCP connections between primary server and backup servers
  * checkpoint_freq to send primary server's state and checkpoitn_count to backup replicas
  * Primary server print the checkpoint messsage sent
  * Backup servers print the received checkpoint message
  * Answer?

* Client
  * Only send the request to primary (?) replica s1 in this milestone
  * Multithreading (refer: https://www.tutorialspoint.com/python/python_multithreading.htm)


#### Milestone D & E
Replica Manager should be responsible for determine server is reboot or not and send the ready messsage to original server or the send_checkpoint message to old server
* Server
  * Active for above R13
  * Add ready parameter that can process following requests from the clients
  * Deal with <server_to_send_checkpoint, server_to_receive_checkpoint, send_checkpoint>
  * Deal with <server_to_be_ready, ready>
  * Deal with "checkpoint" message and set ready to True
  * Flag for active or passive
  
* LFD
  * Deal with <server_to_send_checkpoint, server_to_receive_checkpoint, send_checkpoint> and send to it's server
  * Deal with <server_to_be_ready, ready> and send to it's server

* Client
  * Active
    * The moment client is launched - connect to RM and send message <client_name, client>
    * RM will respond with <client_name, active, S1;S2;S3, ready>
    * Client connect all three servers & send message/wait for replies
    * If server dies, retry to connect to server that is dead.
    * Send messages to servers you are connected to.
  * Passive
    * The moment client is launched - connect to RM and send message <client_name, client>
    * RM will respond with <client_name, passive, primary_server, ready>
    * Client connect to just primary server & send message/wait for replies
    * If primary server dies send rm this message <client_name, client>
    * RM will respond with <client_name, passive, new_primary_server, ready>
    * Send message/wait for replies to new primary server
 * If all three servers not responding after 3 loops. Print servers are dead and kill the client.

* RM
  * Determine the type of overall system (passive or active)
  * Print "RM: 0 members" at first
  * Print "RM: ? members: server_name" for each membership update from GFD
  * Open TCP server for GFD
  * Another list for already registered server and determine the new server should be ready or not to send <server_to_be_ready, ready> to GFD. Or send <server_to_send_checkpoint, server_to_receive_checkpoint, send_checkpoint> to GFD

* GFD
  * Connect to RM by TCP
  * Send membership to RM when membership changed or at the first connection
  * Deal with <server_to_send_checkpoint, server_to_receive_checkpoint, send_checkpoint> and send to corresponding server
  * Deal with <server_to_be_ready, ready> and send to corresponding server

* Active
  1.  Launch RM with active replicas
  2.  Launch GFD
  3.  Launch LFD and
      * LFD will launch corresponding server
      * Server's ready state = 0 at first and response LFD's heartbeat
      * RM would know the membership change from GFD
      * RM send <server_to_be_ready, active, primary, ready>
      * Server get the message from LFD and change ready state = 1 and replica type to active. Print "Server is ready"
  4.  Launch three clients after all three servers are ready
      * Client should connect to RM send the message <client_name, client>
      * RM response with the message <client_name, active, S1;S2;S3,  ready>
  5.  Kill one replica
      * Client should retry to connect to that replica with some frequency
      * RM should know one server is dead and membership change
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>
  6.  Reconnect the replica
      * RM get membership update and assign one replica to send the checkpoint with <**server_to_send_checkpoint**, **server_to_receive_checkpoint**, send_checkpoint>
      * When the server receivethe send_checkpoint message, it would send <**server_to_send_checkpoint**, **server_to_receive_checkpoint**, active, primary, request_id, state, recovery>
      * The recovery server would buffer the requests from the client until it received the recovery message and processing the old requests' id after the request_id and print all the process. Then change ready state to 1
  7.  Kill the replica that recovered
      * RM should know there is membership change
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>
  8.  Kill the second replica
      * Only one replica is alive now and the system should work well
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>

* Passive
  1.  Launch RM with passive replicas
  2.  Launch GFD
  3.  Launch LFD and corresponding server
      * Server's ready state = 0 at first and response LFD's heartbeat
      * RM would know the membership change from GFD
      * RM send <server_to_be_ready, passive, primary/backup, ready>
        * The first server can be assigned the primary replica
      * Server get the message from LFD and change ready state = 1 and replica type to passive. Print "Server is ready"
  4.  Launch three clients after all three servers are ready
      * Client should connect to RM send the message <client_name, client>
      * RM response with the message <client_name, passive, **primary_server**,  ready>
      * Primary replica should send checkpoint message to two backup servers at certain frequency
  5.  Kill primary replica
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>
      * RM should know primary server is dead and membership change. Then assign new primary replica to one server. <new_server, passive, primary, ready>
      * At least some retry to send message to old primary server, client should connect to RM for getting new primary send the message <client_name, client>
      * RM response with the message <client_name, passive, **primary_server**,  ready>
  6.  Recover old primary replica
      * RM would know the membership change from GFD
      * RM send <server_to_be_ready, passive, primary/backup, ready>
      * The new recovered replica would use the checkpoint message from primary server to renew its state after RM's message and set ready state to 1
  7.  Kill the replica that recovered
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>
      * RM should know there is membership change
  8.  Kill the second backup replica
      * RM sends to GFD and GFD sends to LFD <server_to_be_launched, launch>
      * Only one replica is alive now and the system should work well



### Message to communicate
* Request:  
  * format: "<**client_name**,**server_name**,**request_num**,**status_string**,request>" 
  * ex: "<C1, S1, 8, hello_world, request>"
* Reply:  
  * format: "<**client_name**, **server_name**, **request_num**, **status_string**,reply>" 
  * ex: "<C1, S1, 8, hello_world, reply>"
* heartbeat:  
  * format: "<**lfd_name**, **server_name**, **ping_count**, heartbeat>" 
  * ex: "<C1, S1, 8, heartbeat>"
* checkpoint:  
  * format: "<**primary_name**, **backup_name**, **checkpoint_count**, **status_string**, checkpoint>" 
  * ex: "<S1, S2, 8,hello_world, checkpoint>"
* gfd heartbeat:
  * format: "<**gfd_name**, **lfd_name**, **ping_count**, heartbeat>" 
  * ex: "<G, L1, 8, heartbeat>"
* member ship message LFD and GFD:
  * format: "<**lfd_name**, **server_name**, add/delete>, membership" 
  * ex: "<L1, S1, add, membership>" / "<L1, S1, delete, membership>"
* member ship message GFD and RM:
  * format: "<**gfd_name**, **server_name**, add/delete>, membership" 
  * ex: "<GFD, S1, add, membership>" / "<GFD, S1, delete, membership>"
* ready:  
  * format: "<**server_name**, ready>" 
  * ex: "<S1, ready>"
* send_checkpoint
  * format: <**server_to_send_checkpoint**, **server_to_receive_checkpoint**, send_checkpoint>
  *   * ex: "<S1, S2, send_checkpoint>"
* delimiter: ","
* end request: ">"


### Printed Message
* timestamp
  * HH:MM:SS
  
* HeartBeat
  * [timestamp] [heartbeat_count] **sender** sending heartbeat to **receiver**
  * [timestamp] [heartbeat_count] **receiver** receives heartbeat from **sender**

* Checkpoint
  * [timestamp] [checkpoint_count] [my_state] **sender** sending checkpoint to **receiver**
  * [timestamp] [checkpoint_count] [my_state] **receiver** receives checkpoint from **sender**

* Request and response
  * [timestamp] Sent **message_communication without status_string**
  * [timestamp] Received **message_communication without status_string**

* Status
  * [timestamp] my_state_S1 =  ??? before processing **message_communication without status_string**
  * [timestamp] my_state_S1 =  ??? after processing **message_communication without status_string**

* Type
  * [timestamp] [replica_type] [primary/backup] 
