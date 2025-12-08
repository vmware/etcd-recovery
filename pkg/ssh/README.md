## Usage

Run a command via ssh:

```go
package main

import (
	"github.com/vmware/etcd-recovery/pkg/ssh"
	"log"
	"fmt"
)

func main() {

	host := &ssh.Config{
		User:           "root",
		Host:           "192.1.1.3",
		Password:       "123456",
	}
	
	// returns ssh client with above configuration
	client, err := ssh.NewClient(host)
	if err != nil {
		// handle error
	}
	// Defer closing the network connection.
	defer client.Close()

	// Execute your command.
	//  - Run starts a new SSH session and runs the cmd, it returns CombinedOutput and err if any.
	out, err := client.Run("ls /tmp/")

	if err != nil {
		log.Fatal(err)
	}

	// Get your output as []byte.
	fmt.Println(string(out))
}
```

#### Start Connection With Protected Private Key:
```go
host := &ssh.Config{
User:                 "root",
Host:                 "192.1.1.3",
PrivateKeyPath:       "/path/to/your/privateKey",
PrivateKeyPassphrase: "your_private_key_passphrase"
}

// returns ssh client
client, err := ssh.NewClient(host)
if err != nil {
// handle error
}
// Defer closing the network connection.
defer client.Close()
```


#### Start Connection With Private Key:
```go
host := &ssh.Config{
    User:                 "root",
    Host:                 "192.1.1.3",
	PrivateKeyPath:       "/path/to/your/privateKey",
}

// returns ssh client
client, err := ssh.NewClient(host)
if err != nil {
// handle error
}
// Defer closing the network connection.
defer client.Close()
```

#### Start Connection With Password:
```go
host := &ssh.Config{
    User:                 "root",
    Host:                 "192.1.1.3",
    password:             "changeme",
}

// returns ssh client
client, err := ssh.NewClient(host)
if err != nil {
// handle error
}
// Defer closing the network connection.
defer client.Close()
```

#### Upload Local File to Remote:
```go
err := client.Upload("/path/to/local/file", "/path/to/remote/file")
```

#### Download Remote File to Local:
```go
err := client.Download("/path/to/remote/file", "/path/to/local/file")
```

#### Execute Bash Commands:
```go
out, err := client.Run("bash -c 'printenv'")
```

#### Execute Bash Command With Env Variables:
```go
out, err := client.Run(`env MYVAR="MY VALUE" bash -c 'echo $MYVAR;'`)
```

