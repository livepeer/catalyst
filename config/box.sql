--
-- PostgreSQL database dump
--

-- Dumped from database version 14.2 (Debian 14.2-1.pgdg110+1)
-- Dumped by pg_dump version 14.2

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: livepeer; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE livepeer WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE = 'en_US.utf8';


ALTER DATABASE livepeer OWNER TO postgres;

\connect livepeer

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: api_token; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.api_token (
    id character varying(128) NOT NULL,
    data jsonb
);


ALTER TABLE public.api_token OWNER TO postgres;

--
-- Data for Name: api_token; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.api_token (id, data) FROM stdin;
f5914701-fc12-4b3d-b203-b99951e07481	{"id": "f5914701-fc12-4b3d-b203-b99951e07481", "kind": "api-token", "name": "box-key", "userId": "2ac6c34f-f23a-4bb2-8f02-bad095bd38d8", "createdAt": 1646215168581}
\.

CREATE TABLE public.users (
    id character varying(128) NOT NULL,
    data jsonb
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.users (id, data) FROM stdin;
2ac6c34f-f23a-4bb2-8f02-bad095bd38d8	{"id": "2ac6c34f-f23a-4bb2-8f02-bad095bd38d8", "kind": "user", "salt": "939E72D765EA1509", "admin": true, "email": "admin@livepeer.local", "lastName": "User", "lastSeen": 1646215168631, "password": "94820EC94A5E304BFE043ACE41D96BECF80990F153252575FC9D5F6A0F884066", "createdAt": 1646215137901, "firstName": "Box", "emailValid": true, "emailValidToken": "68334acf-57e4-41be-b2de-a73f97015685"}
\.

--
-- Name: api_token api_token_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_token
    ADD CONSTRAINT api_token_pkey PRIMARY KEY (id);

--
-- Name: api_token_userId; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX "api_token_userId" ON public.api_token USING btree (((data ->> 'userId'::text)));
