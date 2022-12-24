import socket
import time
import configparser
import argparse
import sys
from threading import Thread
import datetime
from termcolor import colored

parser = argparse.ArgumentParser()
parser.add_argument('-c', type=str,
                        help='path of configuration')
args = parser.parse_args()

config = configparser.ConfigParser()
config.read(args.c)

HOST1="localhost"
HOST2="172.19.138.17"
HOST3="172.19.138.14"

HOST_LIST = [HOST1, HOST2, HOST3]

PORT = int(config['DEFAULT']['server_port'])

client_name = config['DEFAULT']['client_name']
server_name1 = config['DEFAULT']['server_name1']
server_name2 = config['DEFAULT']['server_name2']
server_name3 = config['DEFAULT']['server_name3']

rm_ip = config['DEFAULT']['rm_ip']
rm_port = int(config['DEFAULT']['rm_port'])

request_num = 0
status_string = config['DEFAULT']['status_string']

try:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect((rm_ip, rm_port))
    print("conntect to the RM")

except socket.error:
    print("Failed to connect to the RM")

passive_retry = 0
shutdown = False
while not shutdown:
    state = ''
    message = '<' + client_name + ',' + 'client' + '>'
    try:
        s.send(message.encode())
        ClientData, clientAddr = s.recvfrom(51200)
        ClientData = ClientData.decode()
        ClientData = ClientData.split(',')

        if ClientData[1] == 'active':
            state = 'active'

            try:
                s1 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                s1.settimeout(10.0)
                s1.connect((HOST1, PORT))
                print("Client socket set")

            except socket.error:
                print("Failed to create S1 socket")

            try:
                s2 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                s2.settimeout(10.0)
                s2.connect((HOST2, PORT))
                print("Client socket set")
            except socket.error:
                print("Failed to create S2 socket")

            try:
                s3 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                s3.settimeout(10.0)
                s3.connect((HOST3, PORT))
                print("Client socket set")
            except socket.error:
                print("Failed to create S3 socket")

            c_dict = dict()
            s_dict = [s1, s2, s3]
            # s_name_dict = [server_name1]
            # s_dict = [s1]
            s_name_dict = [server_name1,server_name2, server_name3]

            def send_message(request_num):
                failures = 0
                for i in range(len(s_dict)):
                    msg_string = "<" + client_name +',' + s_name_dict[i] + ',' +  str(request_num) + ',' + status_string + "_" + client_name + "_"+ str(request_num) +",request" + ">"
                    message = msg_string.encode()
                    try:
                        s_dict[i].send(message)
                        current_time = str(datetime.datetime.now())[11:19]
                        line = status_string + "_" + client_name + "_"+ str(request_num)
                        message = '[' + current_time + ']' + ' Sent ' + "<" + client_name +',' + s_name_dict[i] + ',' +  str(request_num) + ',' + colored(line,'red')  +",request" + ">"
                        print(message)
                    except socket.error:
                        # delete the disconnected server from the dict and retry connection add back to diction when reconnected
                        print("send fail")
                        try:
                            s_dict[i] = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                            s_dict[i].settimeout(10.0)
                            s_dict[i].connect((HOST_LIST[i], PORT))
                            print("reconnect to " + s_name_dict[i])

                        except socket.error:
                            # print("Failed to reconnection")
                            failures += 1
                            continue
                return (failures == len(s_dict))
                        
            def receive_message():
                for i in range(len(s_dict)):
                    # receive 
                    try:
                        ClientData, clientAddr = s_dict[i].recvfrom(51200)
                        ClientData = ClientData.decode()
                        reply_number = ClientData.split(',')[2]
                        server = ClientData.split(',')[1]
                        current_time = str(datetime.datetime.now())[11:19]
                        response = '[' + current_time + ']' +' Received ' + ClientData
                        print(response)
                        if reply_number not in c_dict.keys():
                            c_dict[reply_number] = server
                        else:
                            print('request_num '+ str(reply_number) + ': Discarded duplicate reply from ' + server + '.')
                    except:
                        # delete the disconnected server and retry connection
                        # HIGHLIGHT this
                        print("Did not receive response from " + colored(s_name_dict[i], 'blue'))

            count = 0
            while True:
                request_num+=1

                # create two new threads
                failures = send_message(request_num)
                if failures:
                    count +=1
                else:
                    count = 0
                if count == 3:
                    print("after 3 times retry to connect to servers fail. The client is going to shutdown now")
                    shutdown = True
                    break
                receive_message()

                time.sleep(10)

        elif ClientData[1] == 'passive':
            state = 'passive'
            primary_server = ClientData[2]
            if primary_server == " " or  primary_server == "":
                passive_retry +=1
                if passive_retry == 3:
                    shutdown = True
                    print("after 3 times retry to connect to RM with no primary server. The client is going to shutdown now")
                    break
                time.sleep(10)
                break
            else:
                passive_retry =0
            # HIGHLIGHT this
            print("Primary server is: ", colored(primary_server,'red'))

            if primary_server == 'S1':
                s_name = server_name1
                HOST = HOST1
            elif primary_server == 'S2':
                s_name = server_name2
                HOST = HOST2
            else:
                s_name = server_name3
                HOST = HOST3
            try:
                serv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                serv.connect((HOST, PORT))
                print("conntect to the " + s_name) 
            except socket.error:
                print("Failed to connect to the server")
            request_num = 0

            # dictionary for duplicate reply
            dic = {}
            
            while True:
                request_num+=1
                msg_string = "<" + client_name +',' + s_name + ',' +  str(request_num) + ',' + status_string + "_" + client_name + "_"+ str(request_num) +",request" + ">"
                message = msg_string.encode()

                try:
                    serv.send(message)
                    current_time = str(datetime.datetime.now())[11:19]
                    #HIGHLIGHT STATUS STRING
                    line = status_string + "_" + client_name + "_"+ str(request_num)
                    print('[' + current_time + ']' + ' Sent ' + "<" + client_name +',' + s_name + ',' +  str(request_num) + ',' + colored(line,'red')  +",request" + ">")
                except socket.error:
                    time.sleep(10)
                    # delete the disconnected server from the dict and retry connection add back to diction when reconnected
                    break
                
                # receive 
                try:
                    ClientData, clientAddr = serv.recvfrom(51200)
                    ClientData = ClientData.decode()
                    reply_number = ClientData.split(',')[2]
                    server = ClientData.split(',')[1]
                    current_time = str(datetime.datetime.now())[11:19]
                    response = '[' + current_time + ']' +' Received ' + ClientData
                    print(response)
                except:
                    print("Failed to receive response")
                time.sleep(10)
    except:
        try:
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s.settimeout(10.0)
            s.connect((rm_ip, rm_port))
            print("conntect to the RM")

        except socket.error:
            print("Failed to connect to the RM")
    




