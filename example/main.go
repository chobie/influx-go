package main

import (
	"influx"
	"influx/protocol"

	"fmt"
	"code.google.com/p/goprotobuf/proto"

	"time"
)

func main() {
	client, err := influx.NewTcpClient("localhost", "4649", "root", "root", "debug")
	if err != nil {
		fmt.Printf("errors; %+v", err)
	}
	defer client.Close()

	fmt.Printf("LIST DATABASE:\n")
	databases, _ := client.ListDatabase()
	for _, database := range databases {
		fmt.Printf("db: %s\n", database)
	}
	begin := time.Now()

	for i := 0; i < 1; i++ {
		client.WriteSeries([]*protocol.Series{
			&protocol.Series{
				Name: proto.String("chobie"),
				Fields: []string{"value"},
				Points: []*protocol.Point{
					&protocol.Point{
						Values: []*protocol.FieldValue{
							&protocol.FieldValue{
								DoubleValue: proto.Float64(3.0),
							},
						},
					},
				},
			},
		})
	}

	// client.WriteSeries(series)
	end := time.Now()
	fmt.Printf("Elapsed: %f\n", float32(end.Sub(begin).Nanoseconds()) / 1E9)

	client.Query("select count(value) from chobie")
	client.Ping()

	client.CreateDatabase("ch")
	client.DropDatabase("ch")
}
