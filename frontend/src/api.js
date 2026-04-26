export var API_HOST = "";
export var DEBUG_LOG = false;
if (process.env.NODE_ENV !== 'production') {
  API_HOST = "https://soap.nikscorp.com";
  DEBUG_LOG = true;
}
export const MIN_SEARCH_LEN = 4;


export const onApiCall = (path, resp_ip) => {
  console.log("API CALL", path, resp_ip);
};
