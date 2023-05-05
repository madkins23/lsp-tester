package main

type Message struct {
	JSONRPC string `json:"jsonrpc"`
}

type Request struct {
	Message
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Response struct {
	Message
	ID     int           `json:"id"`
	Result interface{}   `json:"result"`
	Error  ResponseError `json:"error"`
}
