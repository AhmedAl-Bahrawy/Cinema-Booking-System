# Cinema-Booking-System
 Building a Concurrent Cinema Booking System in Golang 

## Rules
- No AI Writes Code
- You Can Ask AI but not copy
- MD files only AI can write it and not for today btw

### Create Ticket System
- System must hold tickets
- Some Cinema Seats
- If one trying to book a seat it will shows for other users
- Do not get in stuck with race conditions
- We Will have two paralel channels (window) at least to book the ticket
- If have time we can Design a good UI for the App


### Focus
- 1st. double booking
- 2nd. handle when "A" user starts booking a seat while "B" user wants to book this seat also (we can use a color to say that is a hold seat)
