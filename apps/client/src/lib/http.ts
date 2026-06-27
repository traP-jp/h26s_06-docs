import axios from "axios";

export const http = axios.create({
    withCredentials: true,
    headers: {
        Accept: "application/json",
    },
});
