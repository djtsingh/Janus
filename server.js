const http = require('http');

const htmlContent = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Protected Page</title>
</head>
<body>
    <h1>Hello from the REAL server!</h1>
    <p>Your request was successfully proxied by Janus.</p>
    <p>Scroll down to test the continuous monitoring feature!</p>
    <div style="height: 1500px;"></div>
    <p>You've reached the bottom.</p>
</body>
</html>
`;

http.createServer((req, res) => {
  res.writeHead(200, {'Content-Type': 'text/html'});
  res.end(htmlContent);
}).listen(3000, () => {
  console.log('Real server running on http://localhost:3000');
});