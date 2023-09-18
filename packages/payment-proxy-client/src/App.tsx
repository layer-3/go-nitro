import * as React from "react";
import { useEffect, useState } from "react";
import Button from "@mui/material/Button";
import CssBaseline from "@mui/material/CssBaseline";
import FormControlLabel from "@mui/material/FormControlLabel";
import Link from "@mui/material/Link";
import Grid from "@mui/material/Grid";
import { createTheme, ThemeProvider } from "@mui/material/styles";
import {
  Alert,
  AlertTitle,
  Slider,
  Stack,
  Switch,
  useMediaQuery,
} from "@mui/material";
import Box from "@mui/material/Box";
import Stepper from "@mui/material/Stepper";
import Step from "@mui/material/Step";
import StepLabel from "@mui/material/StepLabel";
import StepContent from "@mui/material/StepContent";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import PersonIcon from "@mui/icons-material/Person";
import StorageIcon from "@mui/icons-material/Storage";
import { NitroRpcClient } from "@statechannels/nitro-rpc-client";
import { PaymentChannelInfo } from "@statechannels/nitro-rpc-client/src/types";

import {
  QUERY_KEY,
  costPerByte,
  dataSize,
  defaultNitroRPCUrl,
  fileUrl,
  hub,
  initialChannelBalance,
  provider,
} from "./constants";
import { fetchFile } from "./file";
import { Copyright } from "./Copyright";
import { prettyPrintFIL } from "./prettyPrintFIL";

function truncateHexString(h: string) {
  if (h == "") return "";
  return h.slice(0, 6) + "...";
}

export default function App() {
  const prefersDarkMode = useMediaQuery("(prefers-color-scheme: dark)");

  const theme = React.useMemo(
    () =>
      createTheme({
        palette: {
          mode: prefersDarkMode ? "dark" : "light",
        },
      }),
    [prefersDarkMode]
  );

  const url =
    new URLSearchParams(window.location.search).get(QUERY_KEY) ??
    defaultNitroRPCUrl;

  const [nitroClient, setNitroClient] = useState<NitroRpcClient | null>(null);
  const [paymentChannelId, setPaymentChannelId] = useState<string>("");
  const [paymentChannelInfo, setPaymentChannelInfo] = useState<
    PaymentChannelInfo | undefined
  >();

  const [skipPayment, setSkipPayment] = useState(false);

  const [errorText, setErrorText] = useState<string>("");

  useEffect(() => {
    console.time("Connect to Nitro Node");
    NitroRpcClient.CreateHttpNitroClient(url).then(
      (c) => setNitroClient(c),
      (e) => {
        console.error(e);
        setErrorText(e.message);
      }
    );
    console.timeEnd("Connect to Nitro Node");
  }, [url]);

  const updateChannelInfo = async (channelId: string) => {
    if (channelId == "") {
      throw new Error("Empty channel id provided");
    }
    const paymentChannel = await nitroClient?.GetPaymentChannel(channelId);
    setPaymentChannelInfo(paymentChannel);
  };

  const triggerFileDownload = (file: File) => {
    // This will prompt the browser to download the file
    const blob = new Blob([file], { type: file.type });

    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = file.name;
    link.click();
    URL.revokeObjectURL(url);
  };

  const createPaymentChannel = async () => {
    if (!nitroClient) {
      setErrorText("Nitro client not initialized");
      return;
    }
    console.time("Create Payment Channel");
    const result = await nitroClient.CreatePaymentChannel(
      provider,
      [hub],
      initialChannelBalance
    );

    // TODO: If the objective completes fast enough, we might start waiting after it's already done
    // await nitroClient.WaitForObjective(result.Id);

    setPaymentChannelId(result.ChannelId);
    updateChannelInfo(result.ChannelId);
    console.timeEnd("Create Payment Channel");

    // TODO: Slightly hacky but we wait a beat before querying so we see the updated balance
    setTimeout(() => {
      updateChannelInfo(result.ChannelId);
    }, 1000);
  };

  const fetchAndDownloadFile = async () => {
    setErrorText("");

    if (!nitroClient) {
      setErrorText("Nitro client not initialized");
      return;
    }
    if (!paymentChannelInfo) {
      setErrorText("No payment channel to use");
      return;
    }

    try {
      const file = await fetchFile(
        fileUrl,
        skipPayment ? 0 : costPerByte * dataSize,
        paymentChannelInfo.ID,
        nitroClient
      );

      triggerFileDownload(file);

      // TODO: Slightly hacky but we wait a beat before querying so we see the updated balance
      setTimeout(() => {
        updateChannelInfo(paymentChannelInfo.ID);
      }, 50);
    } catch (e: unknown) {
      console.error(e);
      setErrorText((e as Error).message);
    }
  };

  function displayError(errorText: string) {
    if (errorText == "") {
      return <div></div>;
    }
    return (
      <Alert severity="error">
        <AlertTitle>Error</AlertTitle>
        {errorText}
      </Alert>
    );
  }
  const [activeStep, setActiveStep] = React.useState(0);
  const handleNext = () => {
    setActiveStep(activeStep + 1);
  };

  const [createChannelDisabled, setCreateChannelDisabled] = useState(false);
  const [payDisabled, setPayDisabled] = useState(false);

  function VerticalLinearStepper() {
    const handleCreateChannelButton = () => {
      setCreateChannelDisabled(true);
      createPaymentChannel().then(handleNext, (err) => {
        console.log(err);
        setCreateChannelDisabled(false);
      });
    };

    const handlePayButton = () => {
      setPayDisabled(true);
      fetchAndDownloadFile().finally(() => setPayDisabled(false));
    };

    return (
      <Box sx={{ maxWidth: 400 }}>
        <Stepper activeStep={activeStep} orientation="vertical">
          <Step key={"Join the Nitro Payment Network"}>
            <StepLabel>{"Join the Nitro Payment Network"}</StepLabel>
            <StepContent>
              <Typography>{`In this demonstration, you will be sharing in a prefunded network account on Calibration Tesnet with all other users.`}</Typography>
              <Box sx={{ mb: 2 }}>
                <div>
                  <Button
                    disabled={!nitroClient}
                    variant="contained"
                    onClick={handleNext}
                    sx={{ mt: 1, mr: 1 }}
                  >
                    OK
                  </Button>
                </div>
              </Box>
            </StepContent>
          </Step>

          <Step
            key={"Connect to a Retrieval Provider"}
            expanded={!!paymentChannelId}
          >
            <StepLabel>{`Connect to a Retrieval Provider `} </StepLabel>
            <StepContent>
              <Typography>
                {
                  "Create a virtual payment with enough capacity to pay for 10 retrievals."
                }
              </Typography>

              <Box sx={{ mb: 2 }}>
                <Stack direction="row">
                  <Button
                    variant="contained"
                    disabled={createChannelDisabled}
                    onClick={handleCreateChannelButton}
                    sx={{ mt: 1, mr: 1 }}
                  >
                    {paymentChannelId != ""
                      ? truncateHexString(paymentChannelId)
                      : "Create Channel"}
                  </Button>
                </Stack>
              </Box>
              <Stack direction="column">
                <Stack
                  spacing={2}
                  direction="row"
                  sx={{ mb: 1 }}
                  alignItems="center"
                >
                  <PersonIcon />
                  <Slider
                    disabled={!paymentChannelInfo}
                    valueLabelDisplay="off"
                    valueLabelFormat={(value) =>
                      prettyPrintFIL(value) + " remaining"
                    }
                    track="inverted"
                    value={Number(paymentChannelInfo?.Balance.PaidSoFar ?? 0)}
                    min={0}
                    max={Number(
                      (paymentChannelInfo?.Balance.PaidSoFar ?? 0n) +
                        (paymentChannelInfo?.Balance.RemainingFunds ?? 0n)
                    )}
                  />
                  <StorageIcon />{" "}
                </Stack>
                <Typography variant="caption">
                  Funds remaining:{" "}
                  {prettyPrintFIL(paymentChannelInfo?.Balance.RemainingFunds)}
                </Typography>
              </Stack>
            </StepContent>
          </Step>

          <Step key={"Execute a Paid Retrieval"}>
            <StepLabel>{"Execute a Paid Retrieval"}</StepLabel>
            <StepContent>
              <Stack spacing={5} direction="column">
                <Stack>
                  <Typography>
                    {
                      "Create a payment voucher, and attach it to a request for the provider."
                    }
                  </Typography>
                  <Stack direction="column" spacing={2}>
                    <Box
                      component="form"
                      noValidate
                      onSubmit={() => {
                        /* TODO */
                      }}
                      sx={{ mt: 1 }}
                    >
                      <FormControlLabel
                        control={
                          <Switch
                            checked={skipPayment}
                            value="skipPayment"
                            color="primary"
                            onChange={(e) => {
                              setSkipPayment(e.target.checked);
                            }}
                          />
                        }
                        label="Skip payment"
                      />
                      <Button
                        variant="contained"
                        disabled={payDisabled}
                        onClick={handlePayButton}
                        sx={{ mt: 1, mr: 1 }}
                      >
                        Pay & Download
                      </Button>
                    </Box>
                    {displayError(errorText)}
                  </Stack>
                </Stack>
              </Stack>
            </StepContent>
          </Step>
        </Stepper>
      </Box>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <Grid container component="main" sx={{ height: "100vh" }}>
        <CssBaseline />

        <Grid item xs={12} sm={8} md={5} component={Paper} elevation={6} square>
          <Box
            sx={{
              my: 8,
              mx: 4,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
            }}
          >
            <Stack spacing={3}>
              <Typography component="h1" variant="h5">
                Filecoin Paid Retrieval Demo
              </Typography>
              <VerticalLinearStepper />
              <Link
                href="https://statechannels.notion.site/Filecoin-Paid-Retrieval-Demo-bf6ad9ec92a74e139331ce77900305fc?pvs=4"
                variant="body2"
              >
                How does this work?
              </Link>
            </Stack>
            <Copyright sx={{ mt: 5 }} />
          </Box>
        </Grid>
        <Grid
          item
          xs={false}
          sm={4}
          md={7}
          sx={{
            backgroundImage:
              "url(https://source.unsplash.com/random?wallpapers)",
            backgroundRepeat: "no-repeat",
            backgroundColor: (t) =>
              t.palette.mode === "light"
                ? t.palette.grey[50]
                : t.palette.grey[900],
            backgroundSize: "cover",
            backgroundPosition: "center",
          }}
        />
      </Grid>
    </ThemeProvider>
  );
}
