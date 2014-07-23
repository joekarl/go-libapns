go-libapns
==========

APNS library for go

The idea here is to be a simple low level library that will handle establishing a connection and sending push notifications via Apple's apns service with thought towards throughput and performance.

Will handle the latest Apple push notification guidelines at https://developer.apple.com/library/ios/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/ApplePushService.html

Specifically will implement the binary framed format by batching push notifications on a (configurable) timed period.

##Error Handling
As per Apple's guidelines, when a connection is closed due to error, the id of the message which caused the error will be transmitted back over the connection. In this case, multiple push notifications may have followed the bad message. These push notifications will be supplied on a channel **as well as any other unsent messages** and will be then available to re-process.

##Persistent Connection
go-libapns will create a persistent tcp connection to Apple. This allows for the greatest throughput to Apple's servers. On close or error, this connection will be killed and all unsent push notifications will be supplied for re-process.

##Push Notification Length
Apple places a strict limit on push notification length (currently at 256 bytes). go-libapns will attempt to fit your push notification into that size limit by first applying all of your supplied custom fields and applying as much of your alert text as possible. This truncation is not without cost as it takes almost twice the time to fix a message that is too long. 

##Feedback Service
Right now there is no implementation of the feedback service in this library, but one is planned.